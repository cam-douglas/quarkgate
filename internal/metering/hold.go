package metering

// CaptureRelease computes capture and release amounts from hold and actual usage.
func CaptureRelease(holdMicro, captureMicro int64) (capture, release int64) {
	capture = captureMicro
	release = holdMicro - captureMicro
	if release < 0 {
		capture = holdMicro
		release = 0
	}
	return capture, release
}
