package format

import "testing"

func TestResolveColorPolicy(t *testing.T) {
	cases := []struct {
		name     string
		mode     ColorMode
		noFormat bool
		jsonOut  bool
		isTTY    bool
		noColor  bool
		termDumb bool
		want     bool
	}{
		{
			name:     "no-format-disables",
			mode:     ColorAlways,
			noFormat: true,
			isTTY:    true,
			want:     false,
		},
		{
			name:    "json-disables",
			mode:    ColorAlways,
			jsonOut: true,
			isTTY:   true,
			want:    false,
		},
		{
			name:    "no-color-disables",
			mode:    ColorAlways,
			noColor: true,
			isTTY:   true,
			want:    false,
		},
		{
			name:  "auto-requires-tty",
			mode:  ColorAuto,
			isTTY: false,
			want:  false,
		},
		{
			name:     "auto-disables-on-dumb",
			mode:     ColorAuto,
			isTTY:    true,
			termDumb: true,
			want:     false,
		},
		{
			name:  "always-ignores-tty",
			mode:  ColorAlways,
			isTTY: false,
			want:  true,
		},
		{
			name:  "auto-enables",
			mode:  ColorAuto,
			isTTY: true,
			want:  true,
		},
	}

	for _, tc := range cases {
		got := ResolveColorPolicy(tc.mode, tc.noFormat, tc.jsonOut, tc.isTTY, tc.noColor, tc.termDumb).Enabled
		if got != tc.want {
			t.Fatalf("%s: expected %v, got %v", tc.name, tc.want, got)
		}
	}
}
