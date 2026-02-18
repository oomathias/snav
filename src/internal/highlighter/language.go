package highlighter

import "snav/internal/lang"

func DetectLanguage(path string) LangID {
	return lang.Detect(path)
}

func DetectLanguageWithShebang(path string, firstLine string) LangID {
	return lang.DetectWithShebang(path, firstLine)
}
