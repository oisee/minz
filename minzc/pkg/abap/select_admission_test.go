package abap

import (
	"strings"
	"testing"
)

func TestSelectUnsupportedAdditions(t *testing.T) {
	for _, query := range []string{
		"SELECT n FROM t INTO x ORDER BY n DESCENDING. ENDSELECT.",
		"SELECT n FROM t WHERE n > 0 ORDER BY PRIMARY KEY INTO x. ENDSELECT.",
		"SELECT n FROM t INTO x UP TO 1 ROWS. ENDSELECT.",
		"SELECT n FROM t UP TO 1 ROWS INTO x. ENDSELECT.",
		"SELECT DISTINCT n FROM t INTO x. ENDSELECT.",
		"SELECT n FROM t GROUP BY n INTO x. ENDSELECT.",
		"SELECT SINGLE n FROM t WHERE n = @x INTO @y.",
		"SELECT SINGLE n FROM t INTO @y WHERE n = @x.",
		"SELECT SINGLE n FROM t WHERE n = x INTO y.",
		"SELECT SINGLE n FROM t INTO y WHERE n EQ x.",
		"SELECT SINGLE n FROM t WHERE n BETWEEN 0 AND x INTO y.",
		"SELECT SINGLE n FROM t WHERE n IN ( x ) INTO y.",
		"SELECT SINGLE n FROM t WHERE n = 7 + x INTO y.",
	} {
		t.Run(query, func(t *testing.T) {
			m, err := Compile("REPORT zadmit. DATA x TYPE i. DATA y TYPE i. "+query, "zadmit")
			if m != nil || err == nil || !strings.Contains(err.Error(), "SELECT:") {
				t.Fatalf("query admitted or wrong diagnostic: module=%t err=%v", m != nil, err)
			}
			t.Log(err)
		})
	}
}

func TestSelectLiteralClauseWords(t *testing.T) {
	for _, query := range []string{
		"SELECT SINGLE n FROM t WHERE label = 'ORDER BY' INTO @x.",
		"SELECT SINGLE n FROM t INTO @x WHERE label = 'DISTINCT'.",
		"SELECT SINGLE n FROM t WHERE n EQ 7 INTO @x.",
	} {
		if _, err := Compile("REPORT zadmit. DATA x TYPE i. "+query, "zadmit"); err != nil {
			t.Fatalf("literal/scalar query refused: %v", err)
		}
	}
}

func TestSelectUnknownTrailingToken(t *testing.T) {
	_, err := parseSelectStmt([]string{"SELECT", "n", "FROM", "t", "INTO", "(", "x", ")", "UNIMPLEMENTED", "."})
	if err == nil || !strings.Contains(err.Error(), "unsupported clause or token") {
		t.Fatalf("unknown token ignored: %v", err)
	}
}
