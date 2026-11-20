package speed

import (
	"testing"

	"afa/internal/diagnostic"
)

func TestWindowsMeasurementMarkerRejectsSecondWindowsHandle(t *testing.T) {
	firstRelease, err := reserveMeasurement()
	if err != nil {
		t.Fatalf("could not hold Windows measurement marker: %v", err)
	}
	defer func() {
		if firstRelease != nil {
			firstRelease()
		}
	}()
	contenderRelease, err := reserveMeasurement()
	if contenderRelease != nil {
		contenderRelease()
		t.Fatal("a second Windows handle unexpectedly held the marker")
	}
	if diagnostic.CodeOf(err, "") != diagnostic.Busy {
		t.Fatalf("want marker busy, got %v", err)
	}
	firstRelease()
	firstRelease = nil
	secondRelease, err := reserveMeasurement()
	if err != nil {
		t.Fatalf("could not hold marker after first handle closed: %v", err)
	}
	secondRelease()
}
