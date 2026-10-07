// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/scanner"
)

type floatLiteral struct {
	Token string `json:"token"`
	Bits  string `json:"bits"`
}

func floatKind(kind string) bool { return kind == "f32" || kind == "f64" }

var decimalFloat = regexp.MustCompile(`^(?:([0-9]+)(?:\.([0-9]*))?|\.([0-9]+))(?:[eE]([+-]?[0-9]+))?$`)

// Exact rational rounding includes subnormals in the first and only rounding.
// No host floating value participates in literal interpretation.
func exactFloatBits(token, kind string) (string, bool) {
	precision, emin, emax, exponentBits := 24, -126, 127, 8
	if kind == "f64" {
		precision, emin, emax, exponentBits = 53, -1022, 1023, 11
	}
	negative := strings.HasPrefix(token, "-")
	text := strings.TrimPrefix(strings.TrimPrefix(token, "-"), "+")
	text = strings.ReplaceAll(text, "_", "")
	numerator, denominator := new(big.Int), big.NewInt(1)
	if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") || strings.HasPrefix(text, "0b") || strings.HasPrefix(text, "0B") || strings.HasPrefix(text, "0o") || strings.HasPrefix(text, "0O") {
		if _, ok := numerator.SetString(text, 0); !ok {
			return "", false
		}
	} else {
		parts := decimalFloat.FindStringSubmatch(text)
		if parts == nil {
			return "", false
		}
		fraction := parts[2] + parts[3]
		if _, ok := numerator.SetString(parts[1]+fraction, 10); !ok {
			return "", false
		}
		if numerator.Sign() != 0 {
			exponent := new(big.Int)
			if parts[4] != "" {
				if _, ok := exponent.SetString(parts[4], 10); !ok {
					return "", false
				}
			}
			exponent.Sub(exponent, big.NewInt(int64(len(fraction))))
			// Bound physical construction using the exact decimal magnitude.
			order := new(big.Int).Add(exponent, big.NewInt(int64(len(numerator.String())-1)))
			if order.Cmp(big.NewInt(400)) > 0 {
				return "", false
			}
			if order.Cmp(big.NewInt(-400)) < 0 {
				numerator.SetInt64(0)
			} else {
				if !exponent.IsInt64() {
					return "", false
				}
				e := exponent.Int64()
				power := new(big.Int).Exp(big.NewInt(10), big.NewInt(absExponent(e)), nil)
				if e >= 0 {
					numerator.Mul(numerator, power)
				} else {
					denominator.Set(power)
				}
			}
		}
	}
	bits := uint64(0)
	if negative {
		bits = uint64(1) << uint(precision+exponentBits-1)
	}
	if numerator.Sign() == 0 {
		return fmt.Sprintf("%0*x", (precision+exponentBits)/4, bits), true
	}
	exponent := numerator.BitLen() - denominator.BitLen()
	compare := func(e int) int {
		if e >= 0 {
			return numerator.Cmp(new(big.Int).Lsh(new(big.Int).Set(denominator), uint(e)))
		}
		return new(big.Int).Lsh(new(big.Int).Set(numerator), uint(-e)).Cmp(denominator)
	}
	if compare(exponent) < 0 {
		exponent--
	}
	if exponent > emax {
		return "", false
	}
	unit := max(exponent, emin) - (precision - 1)
	n, d := new(big.Int).Set(numerator), new(big.Int).Set(denominator)
	if unit < 0 {
		n.Lsh(n, uint(-unit))
	} else {
		d.Lsh(d, uint(unit))
	}
	q, remainder := new(big.Int), new(big.Int)
	q.QuoRem(n, d, remainder)
	tie := new(big.Int).Lsh(remainder, 1).Cmp(d)
	if tie > 0 || tie == 0 && q.Bit(0) != 0 {
		q.Add(q, big.NewInt(1))
	}
	if q.BitLen() > precision {
		q.Rsh(q, 1)
		exponent++
	}
	if exponent > emax {
		return "", false
	}
	if q.BitLen() == precision {
		field := max(exponent, emin) - emin + 1
		bits |= uint64(field) << uint(precision-1)
		q.Sub(q, new(big.Int).Lsh(big.NewInt(1), uint(precision-1)))
	}
	bits |= q.Uint64()
	return fmt.Sprintf("%0*x", (precision+exponentBits)/4, bits), true
}

func absExponent(e int64) int64 {
	if e < 0 {
		return -e
	}
	return e
}

func floatingNode(file *ast.SourceFile, node *ast.Node) (string, bool) {
	operand, sign := node, ""
	if node.Kind == ast.KindPrefixUnaryExpression {
		prefix := node.AsPrefixUnaryExpression()
		if prefix.Operator != ast.KindMinusToken && prefix.Operator != ast.KindPlusToken {
			return "", false
		}
		operand = prefix.Operand
		if prefix.Operator == ast.KindMinusToken {
			sign = "-"
		} else {
			sign = "+"
		}
	}
	if operand.Kind != ast.KindNumericLiteral {
		return "", false
	}
	start := scanner.SkipTrivia(file.Text(), operand.Pos())
	return sign + file.Text()[start:operand.End()], true
}

// Default decimal/exponent literals have f64 context; integer spellings keep i32.
func floatDefaultHint(file *ast.SourceFile, node *ast.Node) string {
	if token, ok := floatingNode(file, node); ok && strings.ContainsAny(token, ".eE") && !strings.ContainsAny(token, "xX") {
		return "f64"
	}
	return ""
}
