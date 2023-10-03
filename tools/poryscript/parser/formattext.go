package parser

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
)

// FontConfig holds the configuration for various supported fonts, as well as
// the default font.
type FontConfig struct {
	DefaultFontID string           `json:"defaultFontId"`
	Fonts         map[string]Fonts `json:"fonts"`
}

type Fonts struct {
	Widths        map[string]int `json:"widths"`
	MaxLineLength int            `json:"maxLineLength"`
}

// LoadFontConfig reads a font width config JSON file.
func LoadFontConfig(filepath string) (FontConfig, error) {
	var config FontConfig
	bytes, err := ioutil.ReadFile(filepath)
	if err != nil {
		return config, err
	}

	if err := json.Unmarshal(bytes, &config); err != nil {
		return config, err
	}

	return config, err
}

const testFontID = "TEST"

// FormatText automatically inserts line breaks into text
// according to in-game text box widths.
func (fc *FontConfig) FormatText(text string, maxWidth int, fontID string) (string, error) {
	if !fc.isFontIDValid(fontID) && len(fontID) > 0 && fontID != testFontID {
		validFontIDs := make([]string, len(fc.Fonts))
		i := 0
		for k := range fc.Fonts {
			validFontIDs[i] = k
			i++
		}
		return "", fmt.Errorf("unknown fontID '%s' used in format(). List of valid fontIDs are '%s'", fontID, validFontIDs)
	}

	text = strings.ReplaceAll(text, "\n", " ")

	var formattedSb strings.Builder
	var curLineSb strings.Builder
	curWidth := 0
	isFirstLine := true
	isFirstWord := true
	pos := 0
	for pos < len(text) {
		endPos, word, err := fc.getNextWord(text[pos:])
		if err != nil {
			return "", err
		}
		if len(word) == 0 {
			break
		}
		pos += endPos
		if fc.isLineBreak(word) {
			curWidth = 0
			formattedSb.WriteString(curLineSb.String())
			formattedSb.WriteString(word)
			formattedSb.WriteByte('\n')
			if fc.isParagraphBreak(word) {
				isFirstLine = true
			} else {
				isFirstLine = false
			}
			isFirstWord = true
			curLineSb.Reset()
		} else {
			wordWidth := 0
			if !isFirstWord {
				wordWidth += fc.getRunePixelWidth(' ', fontID)
			}
			wordWidth += fc.getWordPixelWidth(word, fontID)
			if curWidth+wordWidth > maxWidth && curLineSb.Len() > 0 {
				formattedSb.WriteString(curLineSb.String())
				if isFirstLine {
					formattedSb.WriteString(`\n`)
					isFirstLine = false
				} else {
					formattedSb.WriteString(`\l`)
				}
				formattedSb.WriteByte('\n')
				isFirstWord = false
				curLineSb.Reset()
				curLineSb.WriteString(word)
				curWidth = wordWidth
			} else {
				curWidth += wordWidth
				if !isFirstWord {
					curLineSb.WriteByte(' ')
				}
				curLineSb.WriteString(word)
				isFirstWord = false
			}
		}
	}

	if curLineSb.Len() > 0 {
		formattedSb.WriteString(curLineSb.String())
	}

	return formattedSb.String(), nil
}

func (fc *FontConfig) getNextWord(text string) (int, string, error) {
	escape := false
	endPos := 0
	startPos := 0
	foundNonSpace := false
	foundRegularRune := false
	endOnNext := false
	controlCodeLevel := 0
	for pos, char := range text {
		if endOnNext {
			return pos, text[startPos:pos], nil
		}
		if escape && (char == 'l' || char == 'n' || char == 'p') {
			if foundRegularRune {
				return endPos, text[startPos:endPos], nil
			}
			endOnNext = true
		} else if char == '\\' && controlCodeLevel == 0 {
			escape = true
			if !foundRegularRune {
				startPos = pos
			}
			foundNonSpace = true
			endPos = pos
		} else {
			if char == ' ' {
				if foundNonSpace && controlCodeLevel == 0 {
					return pos, text[startPos:pos], nil
				}
			} else {
				if !foundNonSpace {
					startPos = pos
				}
				foundRegularRune = true
				foundNonSpace = true
				if char == '{' {
					controlCodeLevel++
				} else if char == '}' {
					if controlCodeLevel > 0 {
						controlCodeLevel--
					}
				}
			}
			escape = false
		}
	}
	if !foundNonSpace {
		return len(text), "", nil
	}
	return len(text), text[startPos:], nil
}

func (fc *FontConfig) isLineBreak(word string) bool {
	return word == `\n` || word == `\l` || word == `\p`
}

func (fc *FontConfig) isParagraphBreak(word string) bool {
	return word == `\p`
}

func (fc *FontConfig) getWordPixelWidth(word string, fontID string) int {
	word, wordWidth := fc.processControlCodes(word, fontID)
	for _, r := range word {
		wordWidth += fc.getRunePixelWidth(r, fontID)
	}
	return wordWidth
}

func (fc *FontConfig) processControlCodes(word string, fontID string) (string, int) {
	width := 0
	re := regexp.MustCompile(`{[^}]*}`)
	positions := re.FindAllStringIndex(word, -1)
	for _, pos := range positions {
		code := word[pos[0]:pos[1]]
		width += fc.getControlCodePixelWidth(code, fontID)
	}
	strippedWord := re.ReplaceAllString(word, "")
	return strippedWord, width
}

func (fc *FontConfig) getRunePixelWidth(r rune, fontID string) int {
	if fontID == testFontID {
		return 10
	}
	return fc.readWidthFromFontConfig(string(r), fontID)
}

func (fc *FontConfig) getControlCodePixelWidth(code string, fontID string) int {
	if fontID == testFontID {
		return 100
	}
	return fc.readWidthFromFontConfig(code, fontID)
}

func (fc *FontConfig) isFontIDValid(fontID string) bool {
	_, ok := fc.Fonts[fontID]
	return ok
}

const fallbackWidth = 0

func (fc *FontConfig) readWidthFromFontConfig(value string, fontID string) int {
	font, ok := fc.Fonts[fontID]
	if !ok {
		return fallbackWidth
	}
	width, ok := font.Widths[value]
	if !ok {
		defaultWidth, ok := font.Widths["default"]
		if !ok {
			return fallbackWidth
		}
		return defaultWidth
	}
	return width
}
