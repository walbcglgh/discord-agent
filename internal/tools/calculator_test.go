package tools

import "testing"

func TestEvalExpr(t *testing.T) {
	cases := map[string]float64{
		"1+2*3":     7,
		"(1+2)*3":   9,
		"-4/2":      -2,
		"10/4":      2.5,
		"2*(3+4)-5": 9,
	}
	for expr, want := range cases {
		got, err := evalExpr(expr)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if got != want {
			t.Errorf("%s: got %v, want %v", expr, got, want)
		}
	}
	if _, err := evalExpr("1/0"); err == nil {
		t.Error("除以零應該回傳錯誤")
	}
}
