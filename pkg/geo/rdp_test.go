package geo

import (
	"testing"
)

func TestSimplifyCoords_StraightLine(t *testing.T) {
	// 5 points exactly on a straight line
	coords := [][2]float64{
		{52.5000, 13.4000},
		{52.5010, 13.4010},
		{52.5020, 13.4020},
		{52.5030, 13.4030},
		{52.5040, 13.4040},
	}

	simplified := SimplifyCoords(coords, 0.0025)

	if len(simplified) != 2 {
		t.Fatalf("expected straight line to simplify from 5 points to 2, got %d", len(simplified))
	}
	if simplified[0] != coords[0] || simplified[1] != coords[4] {
		t.Errorf("expected endpoints preserved: got %v and %v", simplified[0], simplified[1])
	}
}

func TestSimplifyCoords_SharpCornerPreserved(t *testing.T) {
	// L-shaped path: (0,0) -> (0, 0.1) -> (0.1, 0.1)
	coords := [][2]float64{
		{52.5000, 13.4000},
		{52.5000, 13.4050},
		{52.5000, 13.4100}, // Corner at (52.5000, 13.4100)
		{52.5050, 13.4100},
		{52.5100, 13.4100},
	}

	simplified := SimplifyCoords(coords, 0.0025)

	if len(simplified) != 3 {
		t.Fatalf("expected L-shaped path to simplify to 3 points (preserving corner), got %d: %v", len(simplified), simplified)
	}
	if simplified[1] != coords[2] {
		t.Errorf("expected corner point preserved at index 1: got %v, expected %v", simplified[1], coords[2])
	}
}

func TestSimplifyCoords_SmallSlice(t *testing.T) {
	// 0, 1, or 2 points returned as-is
	if res := SimplifyCoords(nil, 0.0025); len(res) != 0 {
		t.Errorf("expected empty result, got %v", res)
	}

	one := [][2]float64{{52.5, 13.4}}
	if res := SimplifyCoords(one, 0.0025); len(res) != 1 {
		t.Errorf("expected 1 point, got %v", res)
	}

	two := [][2]float64{{52.5, 13.4}, {52.6, 13.5}}
	if res := SimplifyCoords(two, 0.0025); len(res) != 2 {
		t.Errorf("expected 2 points, got %v", res)
	}
}
