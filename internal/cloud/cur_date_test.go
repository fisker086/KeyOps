package cloud

import "testing"

func TestCURDayFromValue_int64Millis(t *testing.T) {
	day := CURDayFromValue(int64(1779926400000))
	if day != "2026-05-28" {
		t.Fatalf("got %q want 2026-05-28", day)
	}
}

func TestCURDayFromValue_pointerInt64(t *testing.T) {
	v := int64(1777593600000)
	day := CURDayFromValue(&v)
	if day != "2026-05-01" {
		t.Fatalf("got %q want 2026-05-01", day)
	}
}

func TestCURDayFromValue_intervalString(t *testing.T) {
	day := CURDayFromValue("2026-05-28T00:00:00Z/2026-05-29T00:00:00Z")
	if day != "2026-05-28" {
		t.Fatalf("got %q want 2026-05-28", day)
	}
}

func TestCURDayFromValue_pointerAddressString(t *testing.T) {
	if day := CURDayFromValue("0x58dd7082e460"); day != "" {
		t.Fatalf("pointer address should be ignored, got %q", day)
	}
}

func TestCURRowInMonth(t *testing.T) {
	row := map[string]interface{}{
		"Line_item_usage_start_date": int64(1779926400000),
	}
	if !CURRowInMonth(row, "2026-05") {
		t.Fatal("expected row in 2026-05")
	}
	if CURRowInMonth(row, "2026-06") {
		t.Fatal("expected row not in 2026-06")
	}
}
