"""Shared dev supervisor — spawn, dedupe, and restart local QuarkGate services."""
from __future__ import annotations

import fcntl
import os
import signal
import subprocess
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Callable

ROOT = Path(__file__).resolve().parents[1]  # quarkgate/
REPO = ROOT.parent  # quarkOS/
LOG_DIR = Path(os.environ.get("QUARKGATE_DEV_LOG_DIR", os.path.join(os.environ.get("TMPDIR", "/tmp"), "quarkgate-dev")))
LOCK_DIR = LOG_DIR / "locks"
RESTART_COOLDOWN_SEC = float(os.environ.get("SUPERVISOR_COOLDOWN", "25"))

_last_restart: dict[str, float] = {}


@dataclass(frozen=True)
class GuardedService:
    name: str
    port: int
    patterns: tuple[str, ...]
    start: Callable[[], None] | None = None
    enabled: Callable[[], bool] | None = None


def _env_true(key: str, default: bool = True) -> bool:
    val = os.environ.get(key, "true" if default else "false").lower()
    return val in ("1", "true", "yes", "on")


def _unified_mode() -> bool:
    return _env_true("DEV_UNIFIED", default=False)


def _anytype_bin() -> str:
    for candidate in (
        os.environ.get("ANYTYPE_BIN", ""),
        os.path.expanduser("~/.local/bin/anytype"),
        "/usr/local/bin/anytype",
    ):
        if candidate and Path(candidate).is_file():
            return candidate
    return "anytype"


def _anytype_port() -> int:
    url = os.environ.get("ANYTYPE_API_URL", "http://127.0.0.1:31012")
    if ":" in url.rsplit("/", 1)[-1]:
        return int(url.rsplit(":", 1)[-1])
    return 31012


def _popen_logged(name: str, cmd: str, *, cwd: Path | None = None) -> None:
    LOG_DIR.mkdir(parents=True, exist_ok=True)
    log_path = LOG_DIR / f"{name}.log"
    with open(log_path, "a", encoding="utf-8") as log:
        log.write(f"\n--- supervisor start {time.strftime('%Y-%m-%d %H:%M:%S')} ---\n")
        subprocess.Popen(  # noqa: S603
            ["bash", "-lc", cmd],
            cwd=str(cwd or REPO),
            stdout=log,
            stderr=subprocess.STDOUT,
            start_new_session=True,
        )


def _start_gateway() -> None:
    subprocess.run(["go", "build", "-o", "bin/gateway", "./cmd/gateway"], cwd=ROOT, check=False)  # noqa: S603
    _popen_logged("gateway", "./bin/gateway", cwd=ROOT)


def _start_ledger_worker() -> None:
    subprocess.run(["go", "build", "-o", "bin/ledger-worker", "./cmd/ledger-worker"], cwd=ROOT, check=False)  # noqa: S603
    _popen_logged("ledger-worker", "./bin/ledger-worker", cwd=ROOT)


def _start_task() -> None:
    _popen_logged(
        "task-service",
        "cd services/task-service && TASK_SERVICE_PORT=${TASK_SERVICE_PORT:-8081} python3 run.py",
    )


def _start_hermes() -> None:
    _popen_logged("hermes", "cd services/hermes && python3 run.py")


def _start_memory_curator() -> None:
    _popen_logged("memory-curator", "cd services/memory-curator && python3 run.py")


def _start_anytype() -> None:
    port = _anytype_port()
    bin_path = _anytype_bin()
    _popen_logged("anytype", f"{bin_path} serve --listen-address 127.0.0.1:{port} -q")


def _start_embed() -> None:
    _popen_logged(
        "embed-worker",
        "PYTHONPATH=packages/provider-adapters "
        + str(ROOT)
        + "/services/embed-worker/.venv/bin/uvicorn quark_embed_worker.main:app "
        + f"--app-dir {ROOT}/services/embed-worker --host 127.0.0.1 --port "
        + "${EMBED_WORKER_PORT:-8087}",
        cwd=ROOT,
    )


def _start_sleep() -> None:
    _popen_logged("sleep-worker", "python3 -m quark_memory_sleep_worker.main")


def _start_letta() -> None:
    _popen_logged(
        "letta-bridge",
        "python3 -m uvicorn quark_letta_bridge.main:app --app-dir services/letta-bridge --host 127.0.0.1 --port 8089",
    )


def _start_zep() -> None:
    _popen_logged(
        "zep-ingest",
        "PYTHONPATH=packages/memory-bridge-lib:packages/policy-lib "
        "python3 -m uvicorn quark_zep_ingest.consumer:app --app-dir services/zep-ingest-worker --host 127.0.0.1 --port 8091",
    )


def _start_execution() -> None:
    _popen_logged(
        "execution-controller",
        "EXECUTOR_BACKEND=e2b DOCKER_ENABLED=false "
        "EXECUTION_POLICY_PATH=config/execution_policy.yaml "
        "python3 -m uvicorn quark_execution_controller.main:app "
        "--app-dir services/execution-controller --host 127.0.0.1 --port 8083",
    )


def _start_kuzu() -> None:
    db_path = os.environ.get("KUZU_DB_PATH", "data/kuzu/memory_graph.kuzu")
    venv = ROOT / "services/kuzu-bridge/.venv/bin/uvicorn"
    if not venv.is_file():
        subprocess.run(["bash", str(ROOT / "scripts/setup-kuzu.sh")], cwd=ROOT, check=False)  # noqa: S603
    _popen_logged(
        "kuzu-bridge",
        f"cd {ROOT}/services/kuzu-bridge && . .venv/bin/activate && "
        f"KUZU_ENABLED=true KUZU_DB_PATH={ROOT}/{db_path} "
        "uvicorn quark_kuzu_bridge.main:app --host 127.0.0.1 --port 8093",
        cwd=ROOT,
    )


def _start_dashboard() -> None:
    _popen_logged(
        "dev-dashboard",
        str(ROOT)
        + "/services/dev-dashboard/.venv/bin/uvicorn quark_dev_dashboard.main:app "
        + f"--app-dir {ROOT}/services/dev-dashboard --host 127.0.0.1 --port "
        + "${DEV_DASHBOARD_PORT:-8095}",
        cwd=ROOT,
    )


def services() -> tuple[GuardedService, ...]:
    svcs: list[GuardedService] = [
        GuardedService("quarkgate-gateway", 8080, ("bin/gateway",), _start_gateway, lambda: True),
        GuardedService(
            "ledger-worker",
            9091,
            ("bin/ledger-worker", "ledger-worker"),
            _start_ledger_worker,
            lambda: _env_true("QUARKGATE_LEDGER_WORKER", default=True),
        ),
        GuardedService("task-service", 8081, ("quark_task_service",), _start_task, lambda: True),
        GuardedService(
            "anytype",
            _anytype_port(),
            ("anytype serve",),
            _start_anytype,
            lambda: _env_true("ANYTYPE_ENABLED"),
        ),
        GuardedService(
            "memory-curator",
            8082,
            ("quark_memory_curator", "services/memory-curator/run.py"),
            _start_memory_curator,
            lambda: True,
        ),
        GuardedService(
            "execution-controller",
            8083,
            ("quark_execution_controller",),
            _start_execution,
            lambda: os.environ.get("EXECUTOR_BACKEND", "e2b") == "e2b",
        ),
        GuardedService("hermes", 8084, ("quark_hermes",), _start_hermes, lambda: _env_true("HERMES_ENABLED")),
        GuardedService("embed-worker", 8087, ("quark_embed_worker",), _start_embed, lambda: True),
        GuardedService(
            "sleep-worker",
            8088,
            ("quark_memory_sleep_worker",),
            _start_sleep,
            lambda: _env_true("SLEEP_CYCLE_ENABLED"),
        ),
        GuardedService("letta-bridge", 8089, ("quark_letta_bridge",), _start_letta, lambda: _env_true("LETTA_ENABLED")),
        GuardedService(
            "zep-ingest",
            8091,
            ("quark_zep_ingest",),
            _start_zep,
            lambda: _env_true("ZEP_MODE", default=False) or os.environ.get("ZEP_MODE") == "cloud",
        ),
        GuardedService("kuzu-bridge", 8093, ("quark_kuzu_bridge", "kuzu-bridge"), _start_kuzu, lambda: _env_true("KUZU_ENABLED")),
    ]
    if not _unified_mode():
        svcs.append(
            GuardedService("dev-dashboard", 8095, ("quark_dev_dashboard",), _start_dashboard, lambda: True)
        )
    return tuple(svcs)


def _pids_on_port(port: int) -> list[int]:
    try:
        out = subprocess.run(
            ["lsof", "-nP", f"-iTCP:{port}", "-sTCP:LISTEN", "-t"],
            capture_output=True,
            text=True,
            check=False,
        )
    except FileNotFoundError:
        return []
    return sorted({int(x) for x in out.stdout.split() if x.strip().isdigit()})


def _pids_matching(pattern: str) -> list[int]:
    try:
        out = subprocess.run(["pgrep", "-f", pattern], capture_output=True, text=True, check=False)
    except FileNotFoundError:
        return []
    return sorted({int(x) for x in out.stdout.split() if x.strip().isdigit()})


def _kill(pid: int, sig: int = signal.SIGTERM) -> None:
    try:
        os.kill(pid, sig)
    except ProcessLookupError:
        pass


def dedupe_service(svc: GuardedService, *, verbose: bool = False) -> int:
    killed = 0
    port_pids = _pids_on_port(svc.port)
    if len(port_pids) > 1:
        keeper = port_pids[0]
        for pid in port_pids[1:]:
            if verbose:
                print(f"[supervisor] {svc.name}: port {svc.port} duplicate pid {pid} -> SIGTERM (keep {keeper})")
            _kill(pid)
            killed += 1

    if _pids_on_port(svc.port):
        return killed

    pattern_pids: set[int] = set()
    for pat in svc.patterns:
        if "dev-process-guard" in pat or "quark_dev_unified" in pat:
            continue
        pattern_pids.update(_pids_matching(pat))
    pattern_pids.discard(os.getpid())

    if len(pattern_pids) > 1:
        keeper = min(pattern_pids)
        for pid in sorted(pattern_pids):
            if pid == keeper:
                continue
            if verbose:
                print(f"[supervisor] {svc.name}: duplicate pattern pid {pid} -> SIGTERM (keep {keeper})")
            _kill(pid)
            killed += 1
    return killed


def _acquire_start_lock(name: str):
    LOCK_DIR.mkdir(parents=True, exist_ok=True)
    path = LOCK_DIR / f"{name}.lock"
    fh = open(path, "w", encoding="utf-8")  # noqa: SIM115
    try:
        fcntl.flock(fh, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError:
        fh.close()
        return None
    return fh


def ensure_running(svc: GuardedService, *, verbose: bool = False) -> bool:
    if svc.enabled and not svc.enabled():
        return False
    if svc.start is None:
        return False
    if _pids_on_port(svc.port):
        return False

    now = time.time()
    if now - _last_restart.get(svc.name, 0) < RESTART_COOLDOWN_SEC:
        return False

    lock = _acquire_start_lock(svc.name)
    if lock is None:
        return False
    try:
        if _pids_on_port(svc.port):
            return False
        if verbose:
            print(f"[supervisor] {svc.name}: starting on port {svc.port}")
        svc.start()
        _last_restart[svc.name] = now
        return True
    finally:
        fcntl.flock(lock, fcntl.LOCK_UN)
        lock.close()


def run_once(*, verbose: bool = False) -> int:
    total = 0
    for svc in services():
        total += dedupe_service(svc, verbose=verbose)
    return total


def run_supervise(*, verbose: bool = False) -> int:
    restarted = 0
    for svc in services():
        dedupe_service(svc, verbose=verbose)
        if ensure_running(svc, verbose=verbose):
            restarted += 1
    if verbose and restarted:
        print(f"[supervisor] started/restarted {restarted} service(s)")
    return restarted


def load_env() -> None:
    for env_path in (ROOT / ".env", REPO / ".env"):
        if not env_path.is_file():
            continue
        for line in env_path.read_text(encoding="utf-8").splitlines():
            s = line.strip()
            if not s or s.startswith("#") or "=" not in s:
                continue
            key, _, val = s.partition("=")
            os.environ.setdefault(key.strip(), val.strip().strip('"').strip("'"))


def kill_legacy_dev_processes() -> None:
    """Stop separate dashboard/supervisor terminals from older dev-up."""
    for pat in (
        "dev-process-guard.py --supervise",
        "quark_dev_dashboard.main:app",
        "dev-process-guard.py --watch",
    ):
        for pid in _pids_matching(pat):
            if pid != os.getpid():
                _kill(pid)


def acquire_unified_lock() -> object | None:
    LOG_DIR.mkdir(parents=True, exist_ok=True)
    path = LOG_DIR / "dev-unified.lock"
    fh = open(path, "w", encoding="utf-8")  # noqa: SIM115
    try:
        fcntl.flock(fh, fcntl.LOCK_EX | fcntl.LOCK_NB)
        fh.write(str(os.getpid()))
        fh.flush()
        return fh
    except BlockingIOError:
        fh.close()
        return None
