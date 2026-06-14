package render

import (
	"testing"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestApexLexer(t *testing.T) {
	for _, fname := range []string{"MyTrigger.trigger", "script.apex"} {
		l := lexers.Get(fname)
		if l == nil {
			t.Errorf("no lexer for %s", fname)
		} else if l.Config().Name != "Apex" {
			t.Errorf("%s => got lexer %q, want Apex", fname, l.Config().Name)
		} else {
			t.Logf("OK: %s => %s", fname, l.Config().Name)
		}
	}
}
