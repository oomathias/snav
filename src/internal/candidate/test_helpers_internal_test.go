package candidate

import "fmt"

func makeFixtureCandidates(n int) []Candidate {
	out := make([]Candidate, n)
	for i := 0; i < n; i++ {
		lang := LangGo
		file := fmt.Sprintf("pkg/mod%d/file%d.go", i%100, i%37)
		text := fmt.Sprintf("func Symbol%dHandler(input%d int) int { return input%d + %d }", i, i, i, i%11)

		switch i % 4 {
		case 1:
			lang = LangTypeScript
			file = fmt.Sprintf("src/mod%d/file%d.ts", i%90, i%45)
			text = fmt.Sprintf("export const symbol%dHandler = (input%d: number) => input%d + %d", i, i, i, i%13)
		case 2:
			lang = LangRust
			file = fmt.Sprintf("crates/mod%d/file%d.rs", i%70, i%29)
			text = fmt.Sprintf("pub fn symbol%d_handler(input%d: i64) -> i64 { input%d + %d }", i, i, i, i%17)
		case 3:
			lang = LangPython
			file = fmt.Sprintf("py/mod%d/file%d.py", i%60, i%31)
			text = fmt.Sprintf("def symbol_%d_handler(input_%d): return input_%d + %d", i, i, i, i%19)
		}

		out[i] = Candidate{
			ID:     i + 1,
			File:   file,
			Line:   (i % 400) + 1,
			Col:    1,
			Text:   text,
			Key:    fmt.Sprintf("Symbol%dHandler", i),
			LangID: lang,
		}
	}
	return out
}
