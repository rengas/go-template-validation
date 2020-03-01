package main

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	textTemplate "text/template"
)

var (
	templateErrorRegex    = regexp.MustCompile(`template: (.*?):((\d+):)?(\d+): (.*)`)
	findTokenRegex        = regexp.MustCompile(`"(.+)"`)
	functionNotFoundRegex = regexp.MustCompile(`function "(.+)" not defined`)
)

func createTemplateError(err error, level ErrorLevel) templateError {
	matches := templateErrorRegex.FindStringSubmatch(err.Error())
	if len(matches) == 6 {
		// tplName := matches[1]

		// 2 is line + : group if char is found
		// line is in pos 4, unless a char is found in which case it's 3 and char is 4

		lineIndex := 4
		char := -1
		if matches[3] != "" {
			lineIndex = 3
			char, err = strconv.Atoi(matches[4])
			if err != nil {
				char = -1
			}
		}

		line, err := strconv.Atoi(matches[lineIndex])
		if err != nil {
			line = -1
		} else {
			line = line - 1
		}

		description := matches[5]

		return templateError{
			Line:        line,
			Char:        char,
			Description: description,
			Level:       level,
		}
	}
	return templateError{
		Line:        -1,
		Char:        -1,
		Description: err.Error(),
		Level:       misunderstoodError,
	}
}

func parse(text string, baseTpl *textTemplate.Template, depth int) (*textTemplate.Template, []templateError) {
	lines := strings.Split(strings.Replace(text, "\r\n", "\n", -1), "\n")
	tplErrs := make([]templateError, 0)

	if depth > 10 {
		return baseTpl, tplErrs
	}

	t, err := baseTpl.Parse(text)
	if err != nil {
		tplErr := createTemplateError(err, parseErrorLevel)
		if tplErr.Level != misunderstoodError {
			if tplErr.Char == -1 {
				// try to find a character to line up with
				tokenLoc := findTokenRegex.FindStringIndex(tplErr.Description)
				if tokenLoc != nil {
					token := string(tplErr.Description[tokenLoc[0]+1 : tokenLoc[1]-1])
					lastChar := strings.LastIndex(lines[tplErr.Line], token)
					firstChar := strings.Index(lines[tplErr.Line], token)
					// if it's not the only match, we don't know which character is the one the error occured on
					if lastChar == firstChar {
						tplErr.Char = firstChar
					}
				}
			}
			tplErrs = append(tplErrs, tplErr)

			badFunctionMatch := functionNotFoundRegex.FindStringSubmatch(tplErr.Description)
			if badFunctionMatch != nil {
				token := badFunctionMatch[1]
				t, parseTplErrs := parse(text, baseTpl.Funcs(textTemplate.FuncMap{
					token: func() error {
						return nil
					},
				}), depth+1)
				return t, append(tplErrs, parseTplErrs...)
			}
		}

		return baseTpl, tplErrs
	}
	return t, tplErrs
}

func exec(t *textTemplate.Template, data interface{}, buf *bytes.Buffer) []templateError {
	tplErrs := make([]templateError, 0)
	err := t.Execute(buf, data)
	if err != nil {
		tplErrs = append(tplErrs, createTemplateError(err, execErrorLevel))
	}
	return tplErrs
}