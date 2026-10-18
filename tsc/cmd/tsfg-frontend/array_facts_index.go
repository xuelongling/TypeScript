// SPDX-License-Identifier: Apache-2.0
// Exact source-token Array index admission never rounds through a host float.
package main

import (
	"strconv"
	"strings"
)

// The official parser has already validated this NumericLiteral token's grammar.
// Exponents are compared as digit strings; powers larger than the u32 domain are never built.
func arrayFactsUnsigned(token string, maximum uint64) (string, bool) {
	text := strings.ReplaceAll(token, "_", "")
	if len(text) >= 2 && text[0] == '0' {
		base, width := 0, 0
		switch text[1] {
		case 'x', 'X':
			base, width = 16, 4
		case 'o', 'O':
			base, width = 8, 3
		case 'b', 'B':
			base, width = 2, 1
		}
		if base != 0 {
			digits := strings.TrimLeft(text[2:], "0")
			if digits == "" {
				return "0", true
			}
			first, err := strconv.ParseUint(digits[:1], base, 8)
			if err != nil {
				return "", false
			}
			bits := 0
			for first != 0 {
				bits++
				first >>= 1
			}
			if (len(digits)-1)*width+bits > 32 {
				return "", false
			}
			value, err := strconv.ParseUint(digits, base, 32)
			if err != nil || value > maximum {
				return "", false
			}
			return strconv.FormatUint(value, 10), true
		}
	}
	parts := decimalFloat.FindStringSubmatch(text)
	if parts == nil {
		return "", false
	}
	fraction := parts[2] + parts[3]
	coefficient := strings.TrimLeft(parts[1]+fraction, "0")
	if coefficient == "" {
		return "0", true
	}
	trimmed := strings.TrimRight(coefficient, "0")
	trailing := len(coefficient) - len(trimmed)
	coefficient = trimmed
	exponent := parts[4]
	negative := strings.HasPrefix(exponent, "-")
	exponent = strings.TrimLeft(strings.TrimPrefix(strings.TrimPrefix(exponent, "-"), "+"), "0")
	bound := strconv.Itoa(len(parts[1]) + 2*len(fraction) + 10)
	if len(exponent) > len(bound) || len(exponent) == len(bound) && exponent > bound {
		return "", false
	}
	e := 0
	if exponent != "" {
		var err error
		e, err = strconv.Atoi(exponent)
		if err != nil {
			return "", false
		}
	}
	if negative {
		e = -e
	}
	scale := e - len(fraction) + trailing
	if scale < 0 || len(coefficient) > 10 || scale > 10-len(coefficient) {
		return "", false
	}
	digits := coefficient + strings.Repeat("0", scale)
	value, err := strconv.ParseUint(digits, 10, 32)
	if err != nil || value > maximum {
		return "", false
	}
	return strconv.FormatUint(value, 10), true
}
