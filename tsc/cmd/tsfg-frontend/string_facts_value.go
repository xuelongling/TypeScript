// SPDX-License-Identifier: Apache-2.0
// Official JS-string bytes are decoded before any owned UTF16 allocation.
package main

import (
	"github.com/microsoft/typescript-go/internal/stringutil"
	"unicode/utf16"
	"unicode/utf8"
)

const stringFactsValueUnitLimit = 16384
const stringFactsPackageUnitLimit = 65536

func scanStringFactsValue(value string, emit func(int32)) (int, error) {
	count := 0
	for at := 0; at < len(value); {
		r, size := stringutil.DecodeJSStringRune(value[at:])
		if size <= 0 || size > len(value)-at || r == utf8.RuneError && size == 1 || r < 0 || r > utf8.MaxRune {
			return 0, stringFactsError("unicode", "broken official JS-string bytes", nil)
		}
		width := 1
		if r > 0xffff {
			width = 2
		}
		if count > stringFactsValueUnitLimit-width {
			return 0, stringFactsError("limit", "value UTF16 code units", nil)
		}
		count += width
		if emit != nil {
			if width == 1 {
				emit(int32(r))
			} else {
				high, low := utf16.EncodeRune(r)
				emit(int32(high))
				emit(int32(low))
			}
		}
		at += size
	}
	return count, nil
}
func stringFactsTotalUnits(values []string) (int, error) {
	total := 0
	for _, value := range values {
		count, err := scanStringFactsValue(value, nil)
		if err != nil {
			return 0, err
		}
		if total > stringFactsPackageUnitLimit-count {
			return 0, stringFactsError("limit", "package UTF16 code units", nil)
		}
		total += count
	}
	return total, nil
}
func stringFactsValueUnits(value string) ([]int32, error) {
	count, err := scanStringFactsValue(value, nil)
	if err != nil {
		return nil, err
	}
	units := make([]int32, 0, count)
	_, err = scanStringFactsValue(value, func(unit int32) { units = append(units, unit) })
	if err != nil {
		return nil, err
	}
	return units, nil
}
