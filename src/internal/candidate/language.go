package candidate

import "snav/internal/lang"

func detectLanguage(path string) LangID {
	return lang.Detect(path)
}
