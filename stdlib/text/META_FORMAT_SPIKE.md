# Compile-time formatting spike

[`meta_format_spike.nanz`](spikes/meta_format_spike.nanz) uses a user-defined `fun @make_report` metafunction. At compile time it reads a declarative list of literal text and expressions, then emits one concrete `render_report(game: ^Game)` function. A call to that function writes `score=65535 moves=7` into a bounded byte buffer. The expression list includes fields of a `Game` object.

The generated path calls `CText.write_to` and `Dec16.write_to` directly. [`TestMetaFormatSpike`](../../minzc/pkg/nanz/metafunc_generated_test.go) checks exact MIR2 bytes and the concrete call targets in HIR. There is no runtime format-string parser.

[`meta_inline_print_spike.nanz`](spikes/meta_inline_print_spike.nanz) exercises statement-position metafunction expansion in the caller's scope. [`meta_template_print_spike.nanz`](spikes/meta_template_print_spike.nanz) imports the experimental [`text.template_print`](template_print.nanz) module. Its `@print("score=#{score} game=#{game:show}\\n")` parses the template at compile time and emits calls to `CText`, `Dec16`, and `Game.write_to`. The caller must import `text.print` and provide globals named `writer: BufWriter`, `text_value: CText`, and `number_value: Dec16`. The template defaults to decimal; `:text`, `:char`, and `:show` are explicit alternatives. The remaining API work is typed argument introspection, caller-selected sink and formatter names, and stronger diagnostics for unsupported expressions.

Run the template example through MZV:

```sh
cd minzc
go run ./cmd/mzv -H ../stdlib/text/spikes/meta_template_print_spike.nanz
# score=65535 game=Game(7)
```
