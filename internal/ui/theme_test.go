package ui

import "testing"

func TestApplyThemeKnown(t *testing.T) {
	for _, name := range []string{"default", "light", "solarized", "gruvbox", "shades", "mono", "phosphor"} {
		ApplyTheme(name)
		if currentTheme != name {
			t.Errorf("currentTheme=%q after ApplyTheme(%q)", currentTheme, name)
		}
	}
	// Restore default for subsequent tests.
	ApplyTheme("default")
}

func TestApplyThemeUnknownFallsBackToDefault(t *testing.T) {
	ApplyTheme("nonexistent")
	if currentTheme != "default" {
		t.Errorf("unknown theme should fall back to default, got %q", currentTheme)
	}
}

func TestIsMonoThemeFlag(t *testing.T) {
	ApplyTheme("mono")
	if !isMonoTheme {
		t.Error("isMonoTheme should be true after ApplyTheme(\"mono\")")
	}
	ApplyTheme("default")
	if isMonoTheme {
		t.Error("isMonoTheme should be false after ApplyTheme(\"default\")")
	}
}

func TestVcpuColorsForPhosphorAllGreen(t *testing.T) {
	cs := vcpuColorsFor("phosphor")
	if len(cs) == 0 {
		t.Fatal("phosphor returned no colours")
	}
	// Every phosphor entry must be a 256-colour green-cube index in
	// the 22..154 range — that's what the theme reserves for greens.
	greens := map[string]bool{
		"22": true, "28": true, "34": true, "40": true, "46": true,
		"82": true, "118": true, "154": true,
	}
	for _, c := range cs {
		if !greens[string(c)] {
			t.Errorf("phosphor cycles non-green colour %q", c)
		}
	}
}

func TestVcpuColorsForMonoIsBlank(t *testing.T) {
	cs := vcpuColorsFor("mono")
	for i, c := range cs {
		if string(c) != "" {
			t.Errorf("mono[%d]=%q, want empty", i, c)
		}
	}
}

func TestVcpuColorsForDefaultIsRainbow(t *testing.T) {
	cs := vcpuColorsFor("default")
	if len(cs) < 4 {
		t.Fatalf("rainbow should have several distinct colours, got %d", len(cs))
	}
	seen := map[string]bool{}
	for _, c := range cs {
		seen[string(c)] = true
	}
	if len(seen) < 4 {
		t.Errorf("rainbow has only %d unique colours: %v", len(seen), seen)
	}
}

func TestThemeNamesIsSorted(t *testing.T) {
	names := themeNames()
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Errorf("themeNames not sorted: %v", names)
			break
		}
	}
}
