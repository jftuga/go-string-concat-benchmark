package stringbench

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

var result string // prevent compiler optimizations

var (
	s32 = "abcdefghijklmnopqrstuvwxyz012345"                                 // 32 chars
	s48 = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKL"                 // 48 chars
	s64 = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ01" // 64 chars
	num = 1234567890
)

func FormatWithSprintf(a, b, c string, d int) string {
	result = fmt.Sprintf("%s %s %s %d", a, b, c, d)
	return result
}

func FormatWithConcat(a, b, c string, d int) string {
	result = a + " " + b + " " + c + " " + strconv.Itoa(d)
	return result
}

func FormatWithBuilder(a, b, c string, d int) string {
	var sb strings.Builder
	sb.Grow(len(a) + len(b) + len(c) + 13)
	sb.WriteString(a)
	sb.WriteByte(' ')
	sb.WriteString(b)
	sb.WriteByte(' ')
	sb.WriteString(c)
	sb.WriteByte(' ')
	sb.WriteString(strconv.Itoa(d))
	result = sb.String()
	return result
}

func FormatWithBuilderAppend(a, b, c string, d int) string {
	var sb strings.Builder
	sb.Grow(len(a) + len(b) + len(c) + 13)
	sb.WriteString(a)
	sb.WriteByte(' ')
	sb.WriteString(b)
	sb.WriteByte(' ')
	sb.WriteString(c)
	sb.WriteByte(' ')
	var scratch [20]byte
	sb.Write(strconv.AppendInt(scratch[:0], int64(d), 10))
	result = sb.String()
	return result
}

func FormatWithBuffer(a, b, c string, d int) string {
	var buf bytes.Buffer
	buf.Grow(len(a) + len(b) + len(c) + 13)
	buf.WriteString(a)
	buf.WriteByte(' ')
	buf.WriteString(b)
	buf.WriteByte(' ')
	buf.WriteString(c)
	buf.WriteByte(' ')
	buf.WriteString(strconv.Itoa(d))
	result = buf.String()
	return result
}

func BenchmarkSprintf(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FormatWithSprintf(s32, s48, s64, num)
	}
}

func BenchmarkConcat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FormatWithConcat(s32, s48, s64, num)
	}
}

func BenchmarkBuilder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FormatWithBuilder(s32, s48, s64, num)
	}
}

func BenchmarkBuilderAppend(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FormatWithBuilderAppend(s32, s48, s64, num)
	}
}

func BenchmarkBuffer(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FormatWithBuffer(s32, s48, s64, num)
	}
}
