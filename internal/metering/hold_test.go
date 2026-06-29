package metering

import "testing"

func TestCaptureRelease(t *testing.T) {
	tests := []struct {
		hold, actual, wantCapture, wantRelease int64
	}{
		{1_000_000, 300_000, 300_000, 700_000},
		{1_000_000, 0, 0, 1_000_000},
		{500_000, 500_000, 500_000, 0},
		{100_000, 200_000, 100_000, 0},
	}
	for _, tc := range tests {
		c, r := CaptureRelease(tc.hold, tc.actual)
		if c != tc.wantCapture || r != tc.wantRelease {
			t.Fatalf("hold=%d actual=%d got capture=%d release=%d want %d/%d",
				tc.hold, tc.actual, c, r, tc.wantCapture, tc.wantRelease)
		}
	}
}
