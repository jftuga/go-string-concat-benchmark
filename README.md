# Benchmarking Go String Formatting: Sprintf, Concatenation, strings.Builder, and bytes.Buffer

When you need to assemble a string from multiple pieces in Go, you have options. Most developers reach for `fmt.Sprintf` out of habit, some use the `+` operator, and others swear by `strings.Builder` or `bytes.Buffer`. But which one actually performs best, and why?

I set up a controlled benchmark to find out, and the results challenged some of my assumptions.

## The Setup

The benchmark constructs the same string using four different methods. Each function takes three strings of lengths 32, 48, and 64 characters, plus the integer `1234567890`, and produces a single space-separated result string of roughly 157 characters.

```go
var (
    s32 = "abcdefghijklmnopqrstuvwxyz012345"                                 // 32 chars
    s48 = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKL"                 // 48 chars
    s64 = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ01" // 64 chars
    num = 1234567890
)
```

The four contenders:

```go
// 1. fmt.Sprintf
func FormatWithSprintf(a, b, c string, d int) string {
    result = fmt.Sprintf("%s %s %s %d", a, b, c, d)
    return result
}

// 2. String concatenation
func FormatWithConcat(a, b, c string, d int) string {
    result = a + " " + b + " " + c + " " + strconv.Itoa(d)
    return result
}

// 3. strings.Builder
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

// 4. bytes.Buffer
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
```

The package-level `result` variable prevents the compiler from optimizing away the function calls. Both Builder and Buffer use `Grow()` to pre-allocate capacity, which is the fair way to test them because without it, they'd reallocate as they grow, and we'd be benchmarking user error rather than the tools themselves. It's worth noting that without `Grow()`, `strings.Builder` can actually be slower than `+` due to multiple re-allocations and copies as the internal slice expands. If you don't know the approximate size of the output string, Builder loses its primary performance edge.

The benchmarks were run with `go test -bench=. -benchmem -count=5` on the following environment:

- **Go**: go1.25.6 darwin/arm64
- **Hardware**: MacBook M4 with 32 GB RAM
- **OS**: MacOS Sequoia 15.7.3

## The Results

```
BenchmarkSprintf-10      11,185,216    106.5 ns/op    216 B/op    5 allocs/op
BenchmarkConcat-10       27,083,806     43.1 ns/op    176 B/op    2 allocs/op
BenchmarkBuilder-10      32,000,319     37.2 ns/op    176 B/op    2 allocs/op
BenchmarkBuffer-10       18,342,826     64.8 ns/op    336 B/op    3 allocs/op
```

Ranked by speed:

| Rank | Method | ns/op | B/op | allocs/op |
|------|--------|------:|-----:|----------:|
| 1st | strings.Builder | 37 | 176 | 2 |
| 2nd | Concatenation (+) | 43 | 176 | 2 |
| 3rd | bytes.Buffer | 65 | 336 | 3 |
| 4th | fmt.Sprintf | 107 | 216 | 5 |

## Analysis

### strings.Builder takes first place

The most surprising result is that `strings.Builder` outperforms plain concatenation by about 16%, despite requiring far more lines of code. The two approaches have identical allocation profiles: 2 allocations, 176 bytes.  Therefore, the speed difference comes down to how the work gets done.

The concatenation expression `a + " " + b + " " + c + " " + strconv.Itoa(d)` is actually seven components being joined. Go's `runtime.concatstrings` has to work with pre-built pieces, and `strconv.Itoa(d)` must run first to produce a string before the concatenation can begin.

Builder with `Grow()` takes a more direct path. It allocates a buffer to exactly the right size upfront, then each `WriteString` and `WriteByte` call is just a `copy()` into a known position. The pre-sized `Grow()` call is doing the real work here, turning what could be multiple allocations into a single perfectly-sized buffer.

### bytes.Buffer: the hidden cost of String()

Buffer and Builder look almost identical in terms of their API, and many developers treat them as interchangeable. The benchmark reveals they are not. Buffer uses 336 bytes and 3 allocations compared to Builder's 176 bytes and 2 allocations, and runs about 75% slower.

The reason is `String()`. `strings.Builder` was purpose-built for constructing strings, and its `String()` method can return a string that shares the underlying byte slice without copying, using `unsafe.Pointer` internally. `bytes.Buffer.String()` cannot do this because it must allocate a new string and copy the entire contents every time. For a 157-character result, that means an extra ~160 byte allocation and a full memory copy on every call. That extra allocation and copy accounts for both the 3rd allocation and most of the 28 ns penalty relative to Builder.

That said, `bytes.Buffer` has a capability Builder does not: it implements `io.Reader` in addition to `io.Writer`. If you need to read back what you wrote incrementally, such as streaming data or feeding content into another reader, Buffer is the right tool. Builder is write-only by design.

### fmt.Sprintf: convenience at a cost

Sprintf comes in last at nearly 3x slower than Builder, with 5 allocations and 216 bytes. The overhead comes from multiple sources: parsing the format string byte by byte, dispatching each `%s` and `%d` verb through switch statements, and boxing the arguments into `any` interfaces for the variadic parameter. With larger strings, the compiler's escape analysis can no longer keep the interface boxing on the stack, and the allocation count balloons.

That said, 107 nanoseconds is still fast in absolute terms. For code that runs in a request handler or a CLI tool, the readability of `fmt.Sprintf("%s %s %s %d", a, b, c, d)` is worth the cost.

## Going Further: Eliminating the Last Allocation

Both the Builder and Concat benchmarks share a hidden cost: `strconv.Itoa(d)` allocates a temporary string on the heap before the result is assembled. Can we eliminate it?

`strconv.AppendInt` writes integer digits directly into an existing byte slice, avoiding the intermediate string entirely. By using a stack-allocated scratch buffer, we can feed the digits straight into the Builder:

```go
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
```

The `[20]byte` array is large enough to hold any 64-bit integer (up to 19 digits plus a sign) and lives entirely on the stack.

The results:

```
BenchmarkBuilder-10          32,000,319     37.2 ns/op    176 B/op    2 allocs/op
BenchmarkBuilderAppend-10    38,119,894     31.3 ns/op    160 B/op    1 allocs/op
```

| Method | ns/op | B/op | allocs/op |
|--------|------:|-----:|----------:|
| Builder + AppendInt | 31 | 160 | 1 |
| Builder | 37 | 176 | 2 |

The `Itoa` allocation is gone. One allocation, 160 bytes, 31 nanoseconds which is 16% faster than regular Builder and 3.4x faster than Sprintf. The missing 16 bytes (176 vs 160) is exactly the heap-allocated `Itoa` string that no longer exists: a 10-digit number in a 16-byte allocator size class.

This is a micro-optimization, and for most code the regular Builder approach is perfectly fine. But in hot loops processing millions of operations, eliminating even a small allocation matters. Each heap allocation carries costs beyond its size: allocator bookkeeping, GC scanning pressure, and potential cache disruption. The 6 ns saved here is almost entirely the cost of one avoided trip to the allocator.

## When to Use What

**`strings.Builder` with `Grow()`** is the best choice for performance-sensitive string assembly, especially when you know the approximate output size. It gives you the lowest latency, the fewest allocations, and scales well as strings get longer or the number of pieces increases. For maximum performance, pair it with `strconv.AppendInt` (or `strconv.AppendFloat`, `strconv.AppendBool`) to avoid intermediate allocations when formatting non-string types.

**Concatenation (`+`)** is the best choice for readability when performance isn't critical. It's one line, it's obvious, and it's only 16% slower than the Builder approach. For two to four components in code that doesn't run in a tight loop, this is the pragmatic choice. As the component count grows beyond five, consider switching to Builder.

**`bytes.Buffer`** makes sense when you're already working in a `[]byte` context such as writing to an `io.Writer`, building HTTP responses, processing binary data, or when you need to read back what you wrote via its `io.Reader` implementation. But if your end goal is a string, prefer Builder. They have nearly identical APIs, so switching is straightforward.

**`fmt.Sprintf`** earns its place when you need actual formatting including padding, width specifiers, hex encoding, floating point precision. The overhead is the cost of that flexibility. Don't use it as a string concatenation tool.

## One More Thing

These results were consistent across multiple runs with negligible variance, which tells us they reflect genuine algorithmic differences rather than measurement noise. The benchmark used `count=5` to confirm stability, and no run deviated by more than 2 ns from its group mean. If you're running your own benchmarks, that kind of consistency is what you want to see before drawing conclusions.
