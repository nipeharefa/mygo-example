package main

import "testing"

func nearly(t *testing.T, label string, got, want float32) {
	t.Helper()
	if d := got - want; d < -0.001 || d > 0.001 {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

func TestStageScale(t *testing.T) {
	cases := map[float32]float32{1280: 1, 640: 0.5, 1920: 1.5, 0: 1}
	for width, want := range cases {
		nearly(t, "stageScale", stageScale(width), want)
	}
}

func TestMetricsFor(t *testing.T) {
	m := metricsFor(1280, 40)
	want := slideMetrics{Text: 40, PadX: 72, PadY: 84}
	if m != want {
		t.Errorf("metricsFor(1280, 40) = %+v, want %+v", m, want)
	}

	half := metricsFor(640, 40)
	if half != (slideMetrics{Text: 20, PadX: 36, PadY: 42}) {
		t.Errorf("metricsFor(640, 40) = %+v", half)
	}

	if got := metricsFor(0, 40); got != want {
		t.Errorf("metricsFor(0, 40) = %+v, want the design size", got)
	}
	if got := metricsFor(1280, 90).Text; got != 72 {
		t.Errorf("size 90 = %v, want clamp to 72", got)
	}
	if got := metricsFor(1280, 10).Text; got != 24 {
		t.Errorf("size 10 = %v, want clamp to 24", got)
	}
	if got := metricsFor(1280, 0).Text; got != 40 {
		t.Errorf("size 0 = %v, want the default 40", got)
	}
}
