require "./native_frontend"
require "./export"

# Follow only reachable, native-resolved user specializations. The stdlib stays
# in the host frontend; accepted scalar primitives are lowered by this bridge.
class UserGraph < Crystal::Visitor
  getter defs = [] of Crystal::Def
  getter symbols = {} of UInt64 => String

  def initialize(@path : String)
  end

  def visit(node : Crystal::ASTNode)
    true
  end

  def visit(node : Crystal::Call)
    node.target_defs.try &.each do |f|
      next unless f.location.try(&.filename) == @path
      # This typed marker has an ordinary native reference implementation, but
      # target generation belongs to the Nanz meta VM via an explicit bridge.
      next if f.name == "minz_meta_repeat"
      next if @symbols.has_key?(f.object_id)
      @symbols[f.object_id] = "cr_#{f.name.gsub(/[^a-zA-Z0-9_]/, "_")}_#{@defs.size}"
      @defs << f
      f.body.accept(self)
    end
    true
  end
end

class EscapeScan < Crystal::Visitor
  getter? raises = false

  def initialize(@fallible : Set(UInt64))
  end

  def visit(node : Crystal::ASTNode)
    true
  end

  def visit(node : Crystal::ExceptionHandler)
    node.rescues.try &.each { |r| r.body.accept(self) }
    false
  end

  def visit(node : Crystal::Call)
    @raises = true if node.name == "raise" || node.target_defs.try(&.any? { |f| @fallible.includes?(f.object_id) })
    true
  end
end

class TypedBridge < Bridge
  @fallible = Set(UInt64).new
  @pending = [] of String
  @handler : Crystal::ASTNode? = nil
  @encoded = false
  @temp = 0
  @errors = {} of String => Int32
  @aliases = {} of String => String
  @meta_functions = {} of String => String
  @meta_receipts = [] of JSON::Any

  def initialize(@graph : UserGraph)
    loop do
      before = @fallible.size
      @graph.defs.each do |f|
        scan = EscapeScan.new(@fallible)
        f.body.accept(scan)
        @fallible << f.object_id if scan.raises?
      end
      break if before == @fallible.size
    end
  end

  def scalar(node) : String
    TYPES[node.type?.to_s]? || fail(node, "unsupported inferred type #{node.type?}")
  end

  def outcome_type(f)
    @fallible.includes?(f.object_id) ? "u16" : scalar(f)
  end

  def signature(f)
    fail(f, "instance receivers are outside this scalar iteration") unless f.owner.is_a?(Crystal::Program)
    params = f.args.map { |a| "(#{name(a, a.name)} #{scalar(a)})" }.join(" ")
    {params, outcome_type(f)}
  end

  def emit(manifest_path)
    compiled = {} of String => Tuple(String, String, String)
    @graph.defs.each do |f|
      @locals = {} of String => String
      f.args.each { |a| @locals[a.name] = scalar(a) }
      @ret = scalar(f)
      @encoded = @fallible.includes?(f.object_id)
      fail(f, "fallible scalar iteration requires UInt8 result") if @encoded && @ret != "u8"
      @pending.clear
      @temp = 0
      @handler = nil
      params, ret = signature(f)
      compiled[@graph.symbols[f.object_id]] = {params, ret, body(f.body, true)}
    end
    # No address-taking/linkage/effects in this admitted subset. Compare typed
    # bodies, including parameter names, rather than merging by source spelling.
    canonical = {} of Tuple(String, String, String) => String
    compiled.each do |symbol, code|
      if same = canonical[code]?
        @aliases[symbol] = same
      else
        canonical[code] = symbol
      end
    end
    rewrite = ->(s : String) do
      s.gsub(/\(call (cr_\w+) /) { |m| "(call #{@aliases[$1]? || $1} " }
    end
    declarations = [] of String
    definitions = [] of String
    compiled.each do |symbol, code|
      next if @aliases.has_key?(symbol)
      params, ret, inner = code
      declarations << "(fun #{symbol} (#{params}) #{ret})"
      definitions << "(fun #{symbol} (#{params}) #{ret} #{rewrite.call(inner)})"
    end
    manifest = @graph.defs.map do |f|
      symbol = @graph.symbols[f.object_id]
      {source: f.name, args: f.args.map { |a| scalar(a) }, result: scalar(f),
       symbol: @aliases[symbol]? || symbol, original_symbol: symbol,
       fallible: @fallible.includes?(f.object_id)}
    end
    @meta_functions.each do |symbol, definition|
      declarations << "(fun #{symbol} ((value u8)) u8)"
      definitions << definition
    end
    File.write(manifest_path, {functions: manifest, errors: @errors, merged: @aliases, meta_generators: @meta_receipts}.to_pretty_json)
    (declarations + definitions).join("\n")
  end

  def drain
    result = @pending.join("\n")
    @pending.clear
    result
  end

  def returned(code)
    "(return #{@encoded ? "(cast #{code} u16)" : code})"
  end

  def error_path(code)
    if handler = @handler
      saved = @handler
      @handler = nil
      result = body(handler, true)
      @handler = saved
      result
    else
      "(return #{code})"
    end
  end

  def stmt(node : Crystal::ASTNode, tail = false) : String
    case node
    when Crystal::ExceptionHandler
      fail(node, "only one bare rescue without binding/else/ensure, at function tail, is supported") unless tail && @handler.nil? && node.rescues.try(&.size) == 1 && !node.else && !node.ensure
      rescue_node = node.rescues.not_nil!.first
      fail(node, "typed/bound rescue unsupported") if rescue_node.types || rescue_node.name
      @handler = rescue_node.body
      inner = body(node.body, tail)
      @handler = nil
      inner
    when Crystal::Return
      code, ty = expr(node.exp || fail(node, "scalar return required"))
      expect(node, ty, @ret)
      "#{drain}\n#{returned(code)}"
    when Crystal::Assign
      fail(node, "assignment-valued return unsupported") if tail
      target = node.target
      fail(node, "only local assignment admitted") unless target.is_a?(Crystal::Var)
      key = name(target, target.name)
      code, ty = expr(node.value)
      if old = @locals[key]?
        expect(node, ty, old)
        "#{drain}\n(set #{key} #{code})"
      else
        @locals[key] = ty
        "#{drain}\n(var #{key} #{ty} #{code})"
      end
    when Crystal::If
      cond, ty = expr(node.cond)
      expect(node, ty, "bool")
      prefix = drain
      before = @locals.dup
      yes = body(node.then, tail)
      @locals = before.dup
      no = body(node.else, tail)
      @locals = before
      "#{prefix}\n(if #{cond} (block #{yes}) (block #{no}))"
    when Crystal::While
      fail(node, "while-valued return unsupported") if tail
      cond, ty = expr(node.cond)
      expect(node, ty, "bool")
      fail(node, "fallible loop condition requires explicit ANF loop") unless @pending.empty?
      before = @locals.dup
      inner = body(node.body)
      @locals = before
      "(while #{cond} #{inner})"
    when Crystal::Nop, Crystal::NilLiteral
      fail(node, "Nil-valued scalar return") if tail
      ""
    else
      if node.is_a?(Crystal::Call) && node.name == "raise"
        fail(node, "raise requires one static message and no receiver") unless node.obj.nil? && node.args.size == 1 && node.args.first.is_a?(Crystal::StringLiteral)
        message = node.args.first.as(Crystal::StringLiteral).value
        code = @errors[message] ||= @errors.size + 1
        fail(node, "too many error codes") if code > 255
        return error_path("(cast #{256 + code} u16)")
      end
      code, ty = expr(node)
      prefix = drain
      if tail
        expect(node, ty, @ret)
        "#{prefix}\n#{returned(code)}"
      else
        "#{prefix}\n#{code}"
      end
    end
  end

  def expr(node : Crystal::ASTNode) : Tuple(String, String)
    if node.is_a?(Crystal::Call)
      if expanded = node.expanded
        return expr(expanded)
      end
      if node.name == "minz_meta_repeat"
        return meta_repeat(node)
      end
      if targets = node.target_defs
        fail(node, "runtime union dispatch unsupported") unless targets.size == 1
        target = targets.first
        if symbol = @graph.symbols[target.object_id]?
          fail(node, "receiver/block calls unsupported") if node.obj || node.block || node.block_arg
          args = node.args.map { |a| expr(a)[0] }
          result = "(call #{symbol} #{args.join(" ")})"
          if @fallible.includes?(target.object_id)
            key = "__outcome_#{@temp += 1}"
            @pending << "(var #{key} u16 #{result})"
            prefix = drain
            failure = error_path(key)
            @pending << "#{prefix}\n(if (> #{key} (cast 255 u16)) (block #{failure}))"
            result = "(cast #{key} u8)"
          end
          return {result, scalar(node)}
        end
      end
      if {"//", "%"}.includes?(node.name)
        obj = node.obj || fail(node, "division receiver missing")
        left, ty = expr(obj)
        fail(node, "unsigned division/remainder only") unless {"u8", "u16", "u32"}.includes?(ty) && node.args.size == 1
        right, right_ty = expr(node.args.first)
        expect(node, right_ty, ty)
        return {"(#{node.name == "//" ? "/" : "%"} #{left} #{right})", ty}
      end
      if {"<<", ">>"}.includes?(node.name)
        obj = node.obj || fail(node, "shift receiver missing")
        left, ty = expr(obj)
        fail(node, "unsigned literal shifts only") unless {"u8", "u16", "u32"}.includes?(ty) && node.args.size == 1 && node.args.first.is_a?(Crystal::NumberLiteral)
        count = node.args.first.as(Crystal::NumberLiteral).value.to_i
        fail(node, "shift count outside bit width") unless 0 <= count < (ty == "u8" ? 8 : (ty == "u16" ? 16 : 32))
        return {"(#{node.name} #{left} #{count})", ty}
      end
    end
    super(node)
  end

  def meta_repeat(node : Crystal::Call) : Tuple(String, String)
    fail(node, "meta repeat requires three positional arguments") unless node.obj.nil? && node.args.size == 3 && !node.block && !node.block_arg && !node.named_args
    node.args.each { |a| expect(a, scalar(a), "u8") }
    expect(node, scalar(node), "u8")
    constants = node.args[1..2].map do |a|
      fail(a, "meta repeat count and delta must be compile-time integer literals") unless a.is_a?(Crystal::NumberLiteral)
      a.value.to_i
    end
    count, delta = constants
    fail(node, "meta repeat requires count 0..32 and delta 0..255") unless (0..32).includes?(count) && (0..255).includes?(delta)
    symbol = "meta_repeat_u8_#{count}_#{delta}"
    unless @meta_functions.has_key?(symbol)
      tool = ENV["MINZ_META_BRIDGE"]? || fail(node, "set MINZ_META_BRIDGE to the crystal-meta-bridge executable")
      output = IO::Memory.new
      errors = IO::Memory.new
      status = Process.run(tool, args: ["repeat", count.to_s, delta.to_s], output: output, error: errors)
      fail(node, "Nanz meta bridge failed: #{errors}") unless status.success?
      receipt = JSON.parse(output.to_s)
      fail(node, "Nanz meta bridge protocol/target mismatch") unless receipt["protocol"].as_s == "minz-meta-hir-v1" && receipt["target"].as_s == "z80"
      fail(node, "Nanz meta bridge specialization mismatch") unless receipt["function"].as_s == symbol && receipt["count"].as_i == count && receipt["delta"].as_i == delta && receipt["generated_additions"].as_i == count
      @meta_functions[symbol] = receipt["lanz"].as_s
      @meta_receipts << receipt
    end
    value, ty = expr(node.args.first)
    {"(call #{symbol} #{value})", ty}
  end
end

begin
  path = File.expand_path(ARGV[0])
  compiler = Crystal::Compiler.new
  compiler.no_codegen = true
  compiler.color = false
  result = compiler.compile(Crystal::Compiler::Source.new(path, File.read(path)), "/tmp/crystal-unused")
  graph = UserGraph.new(path)
  result.node.accept(graph)
  puts TypedBridge.new(graph).emit(ARGV[1])
rescue ex
  STDERR.puts ex.message
  exit 1
end
