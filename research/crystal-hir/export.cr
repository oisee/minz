# Native Crystal parser -> a deliberately small, typed Lanz subset -> HIR.
# No regex parser, compiler fork, LLVM or target runtime required.
require "compiler/crystal/syntax"

class Bridge
  TYPES = {"UInt8" => "u8", "UInt16" => "u16", "Int8" => "i8", "Int16" => "i16", "UInt32" => "u32", "Bool" => "bool"}
  OPS   = {"&+" => "+", "&-" => "-", "&*" => "*", "&" => "&", "|" => "|", "^" => "^", "==" => "==", "!=" => "!=", "<" => "<", "<=" => "<=", ">" => ">", ">=" => ">="}
  @locals = {} of String => String
  @functions = {} of String => Crystal::Def
  @ret = ""

  def fail(node, message) : NoReturn
    raise "#{node.location}: #{message}"
  end

  def name(node, value)
    fail(node, "identifier outside spike subset: #{value}") unless value.matches?(/\A[a-zA-Z_][a-zA-Z0-9_]*\z/)
    value
  end

  def type(node : Crystal::ASTNode?) : String
    if node.is_a?(Crystal::Path)
      return TYPES[node.names.join("::")]? || fail(node, "unsupported type #{node}")
    end
    raise "explicit scalar type annotation required (UInt8/UInt16/UInt32/Int8/Int16/Bool)"
  end

  def nodes(node)
    node.is_a?(Crystal::Expressions) ? node.expressions : [node] of Crystal::ASTNode
  end

  def run(root)
    nodes(root).each do |node|
      fail(node, "only top-level def declarations are supported") unless node.is_a?(Crystal::Def)
      name(node, node.name)
      fail(node, "overloads are not supported by the syntax-only spike") if @functions.has_key?(node.name)
      fail(node, "receiver/block/splat/abstract definitions unsupported") if node.receiver || node.block_arg || node.splat_index || node.double_splat || node.abstract? || node.macro_def?
      @functions[node.name] = node
    end
    # Forward signatures keep Lanz calls typed even when a definition comes later.
    declarations = @functions.values.map do |f|
      params = f.args.map do |a|
        fail(a, "default arguments unsupported") if a.default_value
        "(#{name(a, a.name)} #{type(a.restriction)})"
      end
      "(fun #{f.name} (#{params.join(" ")}) #{type(f.return_type)})"
    end
    definitions = @functions.values.map do |f|
      @locals = {} of String => String
      params = f.args.map do |a|
        ty = type(a.restriction)
        @locals[a.name] = ty
        "(#{a.name} #{ty})"
      end
      @ret = type(f.return_type)
      "(fun #{f.name} (#{params.join(" ")}) #{@ret} #{body(f.body, true)})"
    end
    (declarations + definitions).join("\n")
  end

  def expect(node, actual, expected)
    fail(node, "type mismatch: #{actual}, expected #{expected}") unless actual == expected
  end

  def body(node, tail = false) : String
    list = nodes(node)
    list.map_with_index { |s, i| stmt(s, tail && i == list.size - 1) }.join("\n")
  end

  def stmt(node : Crystal::ASTNode, tail = false) : String
    case node
    when Crystal::Return
      value = node.exp || fail(node, "one return value required")
      code, ty = expr(value)
      expect(node, ty, @ret)
      "(return #{code})"
    when Crystal::Assign
      fail(node, "assignment as implicit return unsupported; put the variable last") if tail
      target = node.target
      fail(node, "only local assignments supported") unless target.is_a?(Crystal::Var)
      key = name(target, target.name)
      code, ty = expr(node.value)
      if old = @locals[key]?
        expect(node, ty, old)
        "(set #{key} #{code})"
      else
        @locals[key] = ty
        "(var #{key} #{ty} #{code})"
      end
    when Crystal::If
      cond, ty = expr(node.cond)
      expect(node, ty, "bool")
      # Deliberately do not merge declarations across branches.
      before = @locals.dup
      yes = body(node.then, tail)
      @locals = before.dup
      no = body(node.else, tail)
      @locals = before
      "(if #{cond} (block #{yes}) (block #{no}))"
    when Crystal::While
      fail(node, "while has Nil value; cannot return it as scalar") if tail
      cond, ty = expr(node.cond)
      expect(node, ty, "bool")
      before = @locals.dup
      inner = body(node.body)
      @locals = before
      "(while #{cond} #{inner})"
    when Crystal::Nop
      fail(node, "missing scalar return value") if tail
      ""
    else
      code, ty = expr(node)
      if tail
        expect(node, ty, @ret)
        "(return #{code})"
      else
        code
      end
    end
  end

  def expr(node : Crystal::ASTNode) : Tuple(String, String)
    case node
    when Crystal::NumberLiteral
      ty = node.kind.to_s
      fail(node, "literal type #{ty} unsupported; use an explicit _u8/_u16/_u32/_i8/_i16 suffix") unless {"u8", "u16", "u32", "i8", "i16"}.includes?(ty)
      value = node.value.to_i64
      bounds = {"u8" => {0_i64, 255_i64}, "u16" => {0_i64, 65535_i64}, "u32" => {0_i64, 4294967295_i64}, "i8" => {-128_i64, 127_i64}, "i16" => {-32768_i64, 32767_i64}}
      lo, hi = bounds[ty]
      fail(node, "literal out of range for #{ty}") unless lo <= value <= hi
      {"(cast #{value} #{ty})", ty}
    when Crystal::BoolLiteral
      {node.value.to_s, "bool"}
    when Crystal::Var
      {name(node, node.name), @locals[node.name]? || fail(node, "unknown local #{node.name}")}
    when Crystal::Call
      fail(node, "blocks and named arguments unsupported") if node.block || node.block_arg || node.named_args
      if obj = node.obj
        op = OPS[node.name]? || fail(node, "unsupported operator #{node.name}; use &+/&-/&* for wrapping arithmetic")
        fail(node, "binary operator requires one argument") unless node.args.size == 1
        left, lt = expr(obj)
        right, rt = expr(node.args.first)
        expect(node, rt, lt)
        comparison = {"==", "!=", "<", "<=", ">", ">="}.includes?(op)
        fail(node, "only ==/!= supported for Bool") if lt == "bool" && !{"==", "!="}.includes?(op)
        {"(#{op} #{left} #{right})", comparison ? "bool" : lt}
      else
        f = @functions[node.name]? || fail(node, "unknown function #{node.name}")
        fail(node, "wrong argument count") unless node.args.size == f.args.size
        args = node.args.map_with_index do |a, i|
          code, ty = expr(a)
          expect(a, ty, type(f.args[i].restriction))
          code
        end
        {"(call #{f.name} #{args.join(" ")})", type(f.return_type)}
      end
    else
      fail(node, "unsupported AST node #{node.class}")
    end
  end
end

{% unless flag?(:crystal_semantic) %}
  begin
    path = ARGV.first? || raise "usage: crystal-hir-export source.cr"
    parser = Crystal::Parser.new(File.read(path))
    parser.filename = path
    puts Bridge.new.run(parser.parse)
  rescue ex
    STDERR.puts ex.message
    exit 1
  end
{% end %}
