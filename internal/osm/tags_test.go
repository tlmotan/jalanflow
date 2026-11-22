package osm

import "testing"

func TestLanesPerDirection(t *testing.T) {
	tests := []struct {
		name     string
		tags     map[string]string
		dir      Direction
		want     int
		inferred bool
	}{
		{"explicit forward", map[string]string{"lanes:forward": "3", "lanes": "5"}, Forward, 3, false},
		{"explicit backward", map[string]string{"lanes:backward": "2", "lanes": "5"}, Backward, 2, false},
		{"oneway total", map[string]string{"oneway": "yes", "lanes": "3"}, Forward, 3, false},
		{"two-way split", map[string]string{"lanes": "4"}, Forward, 2, false},
		{"two-way odd rounds down", map[string]string{"lanes": "3"}, Forward, 1, false},
		{"motorway implied oneway", map[string]string{"highway": "motorway", "lanes": "3"}, Forward, 3, false},
		{"untagged", map[string]string{"highway": "primary"}, Forward, 0, true},
		{"garbage", map[string]string{"lanes": "two"}, Forward, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, inf := LanesPerDirection(tc.tags, tc.dir)
			if got != tc.want || inf != tc.inferred {
				t.Fatalf("got (%d,%v), want (%d,%v)", got, inf, tc.want, tc.inferred)
			}
		})
	}
}

func TestMaxSpeedKPH(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"90", 90}, {"60 km/h", 60}, {"60km/h", 60}, {"", 0}, {"none", 0},
		{"30 mph", 0}, {"MY:urban", 0}, {"-5", 0},
	}
	for _, tc := range tests {
		if got := MaxSpeedKPH(map[string]string{"maxspeed": tc.in}); got != tc.want {
			t.Errorf("MaxSpeedKPH(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestFlag(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"yes", true}, {"viaduct", true}, {"no", false}, {"", false},
	}
	for _, tc := range tests {
		if got := Flag(map[string]string{"bridge": tc.in}, "bridge"); got != tc.want {
			t.Errorf("Flag(bridge=%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
