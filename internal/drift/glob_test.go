package drift

import (
	"regexp"
	"testing"
)

func TestGlobToRegex(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{"exact match", "Gemfile", "Gemfile", true},
		{"exact no match", "Gemfile", "Gemfile.lock", false},
		{"exact requires full path", "Gemfile", "app/Gemfile", false},

		{"single star within segment", "*.rb", "foo.rb", true},
		{"single star does not cross slash", "*.rb", "app/foo.rb", false},
		{"single star mid-pattern", "app/*.rb", "app/foo.rb", true},
		{"single star mid-pattern no match other dir", "app/*.rb", "app/sub/foo.rb", false},

		{"double star trailing", "app/**", "app/x.rb", true},
		{"double star trailing nested", "app/**", "app/a/b/c.rb", true},
		{"double star trailing must not match sibling prefix", "app/**", "application.rb", false},
		{"double star trailing must not match bare dir", "app/**", "app", false},

		{"double star leading", "**/foo.rb", "foo.rb", true},
		{"double star leading nested", "**/foo.rb", "a/b/foo.rb", true},
		{"double star leading no match", "**/foo.rb", "foo.rb.bak", false},

		{"double star middle matches zero dirs", "spec/**/*_spec.rb", "spec/foo_spec.rb", true},
		{"double star middle matches nested dirs", "spec/**/*_spec.rb", "spec/models/foo_spec.rb", true},
		{"double star middle no match wrong suffix", "spec/**/*_spec.rb", "spec/foo_test.rb", false},

		{"question mark single char", "file?.txt", "file1.txt", true},
		{"question mark does not match zero chars", "file?.txt", "file.txt", false},
		{"question mark does not cross slash", "a?b", "a/b", false},

		{"exact path deep", "spec/foo/bar_spec.rb", "spec/foo/bar_spec.rb", true},
		{"exact path deep no match", "spec/foo/bar_spec.rb", "spec/foo/baz_spec.rb", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re := regexp.MustCompile(globToRegex(tt.pattern))
			got := re.MatchString(tt.path)
			if got != tt.want {
				t.Errorf("globToRegex(%q).MatchString(%q) = %v, want %v (regex: %s)",
					tt.pattern, tt.path, got, tt.want, re.String())
			}
		})
	}
}

func TestMatchesScope(t *testing.T) {
	patterns := []string{"app/**", "spec/**/*_spec.rb", "Gemfile"}
	matchers := make([]*regexp.Regexp, len(patterns))
	for i, p := range patterns {
		matchers[i] = regexp.MustCompile(globToRegex(p))
	}

	cases := []struct {
		path string
		want bool
	}{
		{"app/services/ai_calls/call.rb", true},
		{"spec/models/foo_spec.rb", true},
		{"Gemfile", true},
		{"application.rb", false},
		{"config/routes.rb", false},
	}
	for _, c := range cases {
		if got := matchesScope(matchers, c.path); got != c.want {
			t.Errorf("matchesScope(%v, %q) = %v, want %v", patterns, c.path, got, c.want)
		}
	}
}
