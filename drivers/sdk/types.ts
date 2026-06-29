/**
 * QuarkGate driver TypeScript interfaces (reference types for contributors).
 */

export interface QuarkGateEnvelope {
  quarkgate_version?: string;
  provider?: string;
  operation?: string;
  payload?: Record<string, unknown>;
  metering_hints?: { estimated_max_credits?: number; priority?: string };
  trace_id?: string;
}

export interface DownstreamRequest {
  url: string;
  method: string;
  headers: Record<string, string>;
  body?: unknown;
  streaming: boolean;
  provider: string;
  operation: string;
  estimate_micro: number;
}

export interface DriverContext {
  envelope: QuarkGateEnvelope;
  credential: string;
  baseURL: string;
}

export interface ParseResponseContext {
  headers: Record<string, string>;
  body: string;
  streaming: boolean;
  envelope?: QuarkGateEnvelope;
}

export interface HealthStatus {
  ok: boolean;
  latency_ms: number;
  message?: string;
}

export interface DriverManifest {
  id: string;
  version: string;
  category: 'llm' | 'scraper' | 'memory' | 'execution' | 'ui';
  operations: Array<{
    id: string;
    method: string;
    path: string;
    streaming?: boolean;
    compat_paths?: string[];
  }>;
  capabilities?: {
    parse_response?: boolean;
    normalize_usage?: boolean;
    async_poll?: boolean;
  };
  pricing?: Record<string, unknown>;
}

export interface QuarkGateDriver {
  prepareRequest(ctx: DriverContext): DownstreamRequest;
  estimateMaxCost(envelope: QuarkGateEnvelope): number;
  normalizeUsage?(raw: Record<string, unknown>, envelope?: QuarkGateEnvelope): Record<string, unknown>;
  parseResponse?(ctx: ParseResponseContext): Record<string, unknown>;
  healthCheck?(ctx: { baseURL: string; credential: string }): Promise<HealthStatus>;
  pollRun?(ctx: { baseURL: string; credential: string; poll_context: Record<string, unknown> }): Promise<{ raw_usage: Record<string, unknown>; done: boolean }>;
}
