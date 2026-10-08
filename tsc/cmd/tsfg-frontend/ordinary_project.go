// SPDX-License-Identifier: Apache-2.0

package main

import "github.com/microsoft/typescript-go/internal/ast"

func ordinaryAssignmentOperator(kind ast.Kind) string {
	switch kind {
	case ast.KindEqualsToken:
		return "="
	case ast.KindPlusEqualsToken:
		return "+="
	case ast.KindMinusEqualsToken:
		return "-="
	case ast.KindAsteriskEqualsToken:
		return "*="
	case ast.KindSlashEqualsToken:
		return "/="
	case ast.KindPercentEqualsToken:
		return "%="
	case ast.KindLessThanLessThanEqualsToken:
		return "<<="
	case ast.KindGreaterThanGreaterThanEqualsToken:
		return ">>="
	case ast.KindGreaterThanGreaterThanGreaterThanEqualsToken:
		return ">>>="
	case ast.KindAmpersandEqualsToken:
		return "&="
	case ast.KindBarEqualsToken:
		return "|="
	case ast.KindCaretEqualsToken:
		return "^="
	case ast.KindAmpersandAmpersandEqualsToken:
		return "&&="
	case ast.KindBarBarEqualsToken:
		return "||="
	case ast.KindQuestionQuestionEqualsToken:
		return "??="
	}
	return ""
}
