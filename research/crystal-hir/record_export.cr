require "./native_frontend"

# This wire format carries semantic types; Go owns target layout and HIR nodes.
class RecordNode
  include JSON::Serializable
  property kind : String
  property ty : String
  property name = ""
  property value = 0_i64
  property op = ""
  property args = [] of RecordNode
  property body = [] of RecordNode
  property yes = [] of RecordNode
  property no = [] of RecordNode

  def initialize(@kind, @ty = "void", @name = "", @args = [] of RecordNode)
  end
end

record RecordField, name : String, ty : String do
  include JSON::Serializable
end

record RecordDecl, name : String, native : String, fields : Array(RecordField), kind : String = "" do
  include JSON::Serializable
end

record RecordFunc, name : String, source : String, owner : String, params : Array(RecordField), ret : String, body : Array(RecordNode), elem : String = "", library : String = "" do
  include JSON::Serializable
end

class RecordGraph < Crystal::Visitor
  BYTE_FACADE = File.real_path(File.join(__DIR__, "bytes.cr"))
  getter defs = [] of Crystal::Def
  getter constructors = {} of UInt64 => Crystal::Def
  getter symbols = {} of UInt64 => String

  def initialize(@path : String)
  end

  def visit(node : Crystal::ASTNode)
    true
  end

  def visit(node : Crystal::Call)
    return true if {"minz_meta_repeat", "minz_meta_layout_byte"}.includes?(node.name)
    node.target_defs.try &.each do |f|
      next unless f.location.try(&.filename) == @path || f.location.try(&.filename) == BYTE_FACADE
      next if @symbols.has_key?(f.object_id) || @constructors.has_key?(f.object_id)
      if f.new?
        @constructors[f.object_id] = f
      else
        @symbols[f.object_id] = "rec_#{f.name.gsub(/[^a-zA-Z0-9_]/, "_")}_#{@defs.size}"
        @defs << f
      end
      f.body.accept(self) unless byte_binding(f)
    end
    true
  end

  def byte_binding(f : Crystal::Def) : String?
    return nil unless f.location.try(&.filename) == BYTE_FACADE
    owner = {"MinzBytes::View" => "ByteView", "MinzBytes::Buffer" => "ByteBuffer", "MinzBytes::Ring" => "ByteRing"}[f.owner.to_s]?
    return nil unless owner
    method = f.name == "initialize" ? "init" : f.name
    admitted = case owner
               when "ByteView" then {"init", "try_get", "try_set", "subview"}
               when "ByteBuffer" then {"init", "clear", "try_push", "view"}
               else {"init", "clear", "try_push", "try_pop", "segments"}
               end
    admitted.includes?(method) ? "#{owner}_#{method}" : nil
  end
end

class InitializerFinder < Crystal::Visitor
  getter defs = [] of Crystal::Def

  def visit(node : Crystal::ASTNode)
    true
  end

  def visit(node : Crystal::Call)
    node.target_defs.try &.each { |f| @defs << f if f.name == "initialize" }
    true
  end
end

class WritesLocal < Crystal::Visitor
  getter? written = false

  def initialize(@name : String)
  end

  def visit(node : Crystal::ASTNode)
    true
  end

  def visit(node : Crystal::Assign)
    target = node.target
    @written = true if target.is_a?(Crystal::Var) && target.name == @name
    true
  end
end

class YieldValues < Crystal::Visitor
  getter values = [] of Crystal::ASTNode

  def visit(node : Crystal::ASTNode)
    true
  end

  def visit(node : Crystal::Yield)
    raise "#{node.location}: generator yield requires one scalar value" unless node.exps.size == 1
    @values << node.exps.first
    true
  end
end

class RecordExport
  PRIMITIVES = {"UInt8" => "u8", "UInt16" => "u16", "Int8" => "i8", "Int16" => "i16", "Bool" => "bool", "Nil" => "void"}
  OPS        = {"&+" => "+", "&-" => "-", "&*" => "*", "&" => "&", "|" => "|", "^" => "^", "==" => "==", "!=" => "!=", "<" => "<", "<=" => "<=", ">" => ">", ">=" => ">="}
  @records = {} of UInt64 => String
  @declarations = [] of RecordDecl
  @owner = ""
  @in_generator = false

  def initialize(@graph : RecordGraph)
  end

  def fail(node, message) : NoReturn
    raise "#{node.location}: #{message}"
  end

  def type(ty : Crystal::Type) : String
    if p = PRIMITIVES[ty.to_s]?
      return p
    end
    case ty
    when Crystal::PointerInstanceType
      elem = type(ty.element_type)
      raise "unsupported pointer element #{elem}" unless {"u8", "u16", "i8", "i16"}.includes?(elem) || elem.starts_with?("record_")
      "ptr:#{elem}"
    when Crystal::StaticArrayInstanceType
      elem = type(ty.element_type)
      raise "only scalar StaticArray storage supported" unless {"u8", "u16", "i8", "i16"}.includes?(elem)
      count = ty.size.as(Crystal::NumberLiteral).value.to_i
      raise "StaticArray length must be 1..256" unless (1..256).includes?(count)
      "array:#{count}:#{elem}"
    when Crystal::ClassType, Crystal::GenericClassInstanceType
      raise "generic records other than Slice are not admitted" if ty.is_a?(Crystal::GenericClassInstanceType) && !ty.to_s.starts_with?("Slice(")
      raise "class/GC objects unsupported: #{ty}" unless ty.struct?
      if name = @records[ty.object_id]?
        return name
      end
      name = "record_#{@records.size}"
      @records[ty.object_id] = name
      if ty.to_s.starts_with?("Slice(")
        pointer_ty = type(ty.all_instance_vars["@pointer"].type)
        raise "borrowed Slice elements must be u8 or u16" unless {"ptr:u8", "ptr:u16"}.includes?(pointer_ty)
        fields = [RecordField.new("data", pointer_ty), RecordField.new("len", "u16")]
        @declarations << RecordDecl.new(name, ty.to_s, fields, "borrowed_view")
        return name
      end
      fields = ty.all_instance_vars.map do |key, field|
        field_ty = type(field.type)
        raise "unsupported record field type #{field_ty}" unless {"u8", "u16", "i8", "i16", "bool"}.includes?(field_ty) || field_ty.starts_with?("ptr:")
        RecordField.new(key.lchop('@'), field_ty)
      end
      raise "empty structs unsupported" if fields.empty?
      @declarations << RecordDecl.new(name, ty.to_s, fields)
      name
    else
      raise "unsupported semantic type #{ty}"
    end
  end

  def nt(node)
    type(node.type)
  end

  def nodes(node)
    node.is_a?(Crystal::Expressions) ? node.expressions : [node] of Crystal::ASTNode
  end

  def body(node)
    nodes(node).map { |n| encode(n) }
  end

  def condition(node)
    value = encode(node)
    return value if nt(node) == "bool"
    # Crystal considers Nil, false and null pointers falsey. Numeric zero is
    # truthy; evaluating any condition can still have effects.
    RecordNode.new("truthy", "bool", args: [value])
  end

  def receiver(node : Crystal::ASTNode?)
    return RecordNode.new("self", "ptr:#{@owner}") unless node
    if node.is_a?(Crystal::Var) && node.name == "self"
      return RecordNode.new("self", "ptr:#{@owner}")
    end
    fail(node, "method receiver must be an addressable local struct") unless node.is_a?(Crystal::Var) && nt(node).starts_with?("record_")
    RecordNode.new("addr", "ptr:#{nt(node)}", node.name)
  end

  def borrowed_slice_args(slice : Crystal::Call)
    fail(slice, "slice constructor requires pointer and length") unless slice.args.size == 2 && !slice.block && !slice.named_args
    ptr = encode(slice.args[0])
    count = encode(slice.args[1])
    elem = {"ptr:u8" => "u8", "ptr:u16" => "u16"}[ptr.ty]? || fail(slice, "slice each requires unsigned byte/word pointer")
    fail(slice, "slice length must be u8 or u16") unless {"u8", "u16"}.includes?(count.ty)
    ctors = slice.target_defs || fail(slice, "unresolved slice constructor")
    fail(slice, "only borrowed native Slice constructor is admitted") unless ctors.size == 1 && ctors.first.location.try { |loc| loc.filename.to_s.ends_with?("/slice.cr") }
    {ptr, count, elem}
  end

  def slice_parts(value : Crystal::ASTNode)
    if value.is_a?(Crystal::Call) && value.name == "new"
      return borrowed_slice_args(value)
    end
    fail(value, "slice iteration requires an inline constructor or local view") unless value.is_a?(Crystal::Var)
    name = nt(value)
    decl = @declarations.find { |d| d.name == name } || fail(value, "missing borrowed view declaration")
    fail(value, "receiver is not a borrowed view") unless decl.kind == "borrowed_view"
    ptr = RecordNode.new("field", decl.fields[0].ty, "data", [receiver(value)])
    count = RecordNode.new("field", "u16", "len", [receiver(value)])
    {ptr, count, ptr.ty.lchop("ptr:")}
  end

  # Use common foreach when the descriptor is stable; retain live fields
  # through ordinary HIR while lowering when the block rebinds its view.
  def slice_each(node : Crystal::Call) : RecordNode?
    return nil unless node.name == "each" && (block = node.block) && (slice = node.obj)
    return nil unless slice.type.to_s.starts_with?("Slice(")
    fail(node, "slice each requires one block arg") unless block.args.size == 1 && node.args.empty? && !node.named_args && !node.block_arg
    targets = node.target_defs || fail(node, "unresolved slice each")
    fail(node, "only native Indexable each is admitted") unless targets.size == 1 && targets.first.location.try { |loc| loc.filename.to_s.ends_with?("/indexable.cr") }
    ptr, count, elem = slice_parts(slice)
    fail(node, "slice block element type mismatch") unless nt(block.args.first) == elem
    mutable = false
    if slice.is_a?(Crystal::Var)
      writes = WritesLocal.new(slice.name)
      block.body.accept(writes)
      mutable = writes.written?
    end
    result = if mutable
               RecordNode.new("foreach_view", "void", block.args.first.name, [receiver(slice)])
             else
               RecordNode.new("foreach", "void", block.args.first.name, [ptr, count])
             end
    result.op = elem
    result.body = body(block.body)
    result
  end

  def generator_element(f : Crystal::Def) : String?
    scan = YieldValues.new
    f.body.accept(scan)
    return nil if scan.values.empty?
    elem = nt(scan.values.first)
    raise "generator yields must share a scalar type" unless {"u8", "u16", "bool", "i8", "i16"}.includes?(elem) && scan.values.all? { |v| nt(v) == elem }
    raise "generator has too many yield sites" if scan.values.size > 254
    elem
  end

  def generator_each(node : Crystal::Call) : RecordNode?
    block = node.block
    return nil unless block
    targets = node.target_defs || fail(node, "unresolved block call")
    return nil unless targets.size == 1
    f = targets.first
    symbol = @graph.symbols[f.object_id]?
    return nil unless symbol
    elem = generator_element(f)
    return nil unless elem
    fail(node, "nested generator consumption is not yet admitted") if @in_generator
    fail(node, "yielding methods require top-level owner and one block arg") unless f.owner.is_a?(Crystal::Program) && block.args.size == 1 && node.obj.nil? && !node.named_args && !node.block_arg
    fail(node, "generator block element mismatch") unless nt(block.args.first) == elem
    result = RecordNode.new("gen_for", "void", symbol, node.args.map { |a| encode(a) })
    result.op = block.args.first.name
    result.body = body(block.body)
    result
  end

  def encode(node : Crystal::ASTNode) : RecordNode
    case node
    when Crystal::Expressions
      result = RecordNode.new("block")
      result.body = body(node)
      result
    when Crystal::Assign
      fail(node, "yield result feedback is not admitted") if node.value.is_a?(Crystal::Yield)
      RecordNode.new("assign", nt(node), args: [encode(node.target), encode(node.value)])
    when Crystal::Var
      return RecordNode.new("self", "ptr:#{@owner}") if node.name == "self"
      RecordNode.new("var", nt(node), node.name)
    when Crystal::InstanceVar
      fail(node, "instance variable without struct receiver") if @owner.empty?
      RecordNode.new("field", nt(node), node.name.lchop('@'), [RecordNode.new("self", "ptr:#{@owner}")])
    when Crystal::NumberLiteral
      result = RecordNode.new("int", nt(node))
      result.value = node.value.to_i64
      result
    when Crystal::BoolLiteral
      result = RecordNode.new("int", "bool")
      result.value = node.value ? 1_i64 : 0_i64
      result
    when Crystal::NilLiteral, Crystal::Nop
      RecordNode.new("nil")
    when Crystal::Yield
      fail(node, "yield outside imported generator") unless @in_generator && node.exps.size == 1
      RecordNode.new("yield", "void", args: [encode(node.exps.first)])
    when Crystal::Return
      exp = node.exp
      RecordNode.new("return", "void", args: exp ? [encode(exp)] : [] of RecordNode)
    when Crystal::If
      # Statement-valued if may have a Nil union even in a scalar function.
      # Its branches, rather than that unused union value, own tail returns.
      result = RecordNode.new("if", "void", args: [condition(node.cond)])
      result.yes = body(node.then)
      result.no = body(node.else)
      result
    when Crystal::While
      result = RecordNode.new("while", "void", args: [condition(node.cond)])
      result.body = body(node.body)
      result
    when Crystal::Break, Crystal::Next
      exp = node.exp
      fail(node, "valued break/next unsupported") unless exp.nil? || exp.is_a?(Crystal::NilLiteral) || exp.is_a?(Crystal::Nop)
      RecordNode.new(node.is_a?(Crystal::Break) ? "break" : "continue")
    when Crystal::PointerOf
      fail(node, "pointerof in methods requires escape analysis") unless @owner.empty?
      exp = node.exp
      fail(node, "pointerof requires a local variable") unless exp.is_a?(Crystal::Var)
      RecordNode.new("addr", nt(node), exp.name)
    when Crystal::Call
      if expanded = node.expanded
        return encode(expanded)
      end
      if iteration = slice_each(node)
        return iteration
      end
      if consumption = generator_each(node)
        return consumption
      end
      fail(node, "unsupported block call") if node.block
      # size is Int32 in native Slice. Admitted u8/u16 lengths make this
      # explicit checked conversion exact on every accepted constructor.
      if node.name == "to_u16" && node.args.empty? && (size = node.obj).is_a?(Crystal::Call) && size.name == "size" && size.args.empty? && (view = size.obj) && view.type.to_s.starts_with?("Slice(")
        if view.is_a?(Crystal::Call)
          ptr, count, _ = borrowed_slice_args(view)
          return RecordNode.new("view_project", "u16", "len", [ptr, count])
        end
        return slice_parts(view)[1]
      end
      result_ty = nt(node)
      if node.name == "new" && node.type.to_s.starts_with?("Slice(")
        ptr, count, _ = borrowed_slice_args(node)
        return RecordNode.new("construct_view", result_ty, args: [ptr, count])
      end
      if node.name == "new" && result_ty.starts_with?("array:")
        fail(node, "automatic arrays in methods require escape analysis") unless @owner.empty?
        fail(node, "StaticArray.new requires one literal fill value") unless node.args.size == 1 && node.args.first.is_a?(Crystal::NumberLiteral) && !node.block
        return RecordNode.new("array", result_ty, args: [encode(node.args.first)])
      end
      if node.name == "minz_meta_repeat"
        fail(node, "meta repeat requires u8 value and literal u8 count/delta") unless node.args.size == 3 && result_ty == "u8" && node.args.all? { |a| nt(a) == "u8" } && node.args[1..2].all?(&.is_a?(Crystal::NumberLiteral))
        return RecordNode.new("meta_repeat", "u8", args: node.args.map { |a| encode(a) })
      end
      if node.name == "minz_meta_layout_byte"
        fail(node, "meta layout requires pointerof local record and literal field") unless node.args.size == 2 && result_ty == "u8" && node.args[0].is_a?(Crystal::PointerOf) && node.args[1].is_a?(Crystal::StringLiteral)
        # v1 native reference is the marker getter; don't claim arbitrary
        # byte-field reflection until its native fallback is generalized.
        fail(node, "meta layout v1 admits only the marker field") unless node.args[1].as(Crystal::StringLiteral).value == "marker"
        value = node.args[0].as(Crystal::PointerOf).exp
        fail(node, "meta layout requires local record") unless value.is_a?(Crystal::Var) && nt(value).starts_with?("record_") && value.name != "self"
        result = RecordNode.new("meta_layout_byte", "u8", node.args[1].as(Crystal::StringLiteral).value, [receiver(value)])
        result.op = nt(value)
        return result
      end
      targets = node.target_defs || fail(node, "unresolved call")
      fail(node, "union dispatch unsupported") unless targets.size == 1
      f = targets.first
      if ctor = @graph.constructors[f.object_id]?
        finder = InitializerFinder.new
        ctor.body.accept(finder)
        fail(node, "constructor must resolve to one registered initialize") unless finder.defs.size == 1
        init = finder.defs.first
        symbol = @graph.symbols[init.object_id]? || fail(node, "initialize outside user source")
        fail(node, "constructor argument adaptation unsupported") unless ctor.args.map(&.name) == init.args.map(&.name)
        return RecordNode.new("construct", result_ty, symbol, node.args.map { |a| encode(a) })
      end
      if symbol = @graph.symbols[f.object_id]?
        args = node.args.map { |a| encode(a) }
        unless f.owner.is_a?(Crystal::Program)
          args.unshift(receiver(node.obj))
        end
        return RecordNode.new("call", result_ty, symbol, args)
      end
      if node.name == "to_unsafe" && (obj = node.obj) && obj.type.to_s.starts_with?("Slice(")
        if obj.is_a?(Crystal::Call)
          ptr, count, _ = borrowed_slice_args(obj)
          return RecordNode.new("view_project", nt(node), "data", [ptr, count])
        end
        return slice_parts(obj)[0]
      end
      if node.name == "to_unsafe" && (obj = node.obj)
        fail(node, "to_unsafe requires a local StaticArray") unless obj.is_a?(Crystal::Var) && nt(obj).starts_with?("array:")
        return RecordNode.new("addr", result_ty, obj.name)
      end
      if node.name == "null" && result_ty.starts_with?("ptr:")
        return RecordNode.new("int", result_ty)
      end
      obj = node.obj || fail(node, "unsupported free call #{node.name}")
      if node.name == "to_u16" && nt(obj) == "u8" && node.args.empty?
        return RecordNode.new("cast", "u16", args: [encode(obj)])
      end
      if node.name == "to_u8!" && nt(obj) == "u16" && node.args.empty?
        return RecordNode.new("cast", "u8", args: [encode(obj)])
      end
      if nt(obj).starts_with?("ptr:")
        case node.name
        when "+"
          return RecordNode.new("ptr_add", result_ty, args: [encode(obj), encode(node.args.first)])
        when "[]", "[]="
          args = [encode(obj)] + node.args.map { |a| encode(a) }
          return RecordNode.new(node.name == "[]" ? "index" : "store", result_ty, args: args)
        when "null?"
          result = RecordNode.new("binary", "bool", args: [encode(obj), RecordNode.new("int", nt(obj))])
          result.op = "=="
          return result
        end
      end
      op = OPS[node.name]? || fail(node, "unsupported primitive #{node.name}")
      fail(node, "binary primitive arity") unless node.args.size == 1
      result = RecordNode.new("binary", result_ty, args: [encode(obj), encode(node.args.first)])
      result.op = op
      result
    else
      fail(node, "unsupported record AST #{node.class}")
    end
  end

  def emit
    functions = @graph.defs.map do |f|
      @owner = f.owner.is_a?(Crystal::Program) ? "" : type(f.owner)
      raise "instance receivers must be user records" unless @owner.empty? || @owner.starts_with?("record_")
      params = f.args.map do |a|
        ty = nt(a)
        raise "Nil parameters unsupported" if ty == "void"
        raise "aggregate parameters are not yet admitted" if ty.starts_with?("record_") || ty.starts_with?("array:")
        RecordField.new(a.name, ty)
      end
      elem = generator_element(f)
      raise "generator instance methods are not yet admitted" if elem && !@owner.empty?
      @in_generator = !elem.nil?
      ret = elem && f.type.to_s == "NoReturn" ? "void" : nt(f)
      raise "yielding method must return Nil or never return" if elem && ret != "void"
      raise "aggregate returns unsupported" if ret.starts_with?("record_") || ret.starts_with?("array:")
      raise "top-level pointer returns require escape analysis" if @owner.empty? && ret.starts_with?("ptr:")
      library = @graph.byte_binding(f) || ""
      RecordFunc.new(@graph.symbols[f.object_id], f.name, @owner, params, ret, library.empty? ? body(f.body) : [] of RecordNode, elem || "", library)
    end
    {protocol: "crystal-record-hir-v1", records: @declarations, functions: functions}.to_pretty_json
  end
end

begin
  # /tmp and /private/tmp may name the same worktree. File identity, not a
  # symlink spelling, controls which facade methods are admitted.
  path = File.real_path(ARGV[0])
  compiler = Crystal::Compiler.new
  compiler.no_codegen = true
  compiler.color = false
  result = compiler.compile(Crystal::Compiler::Source.new(path, File.read(path)), "/tmp/crystal-unused")
  graph = RecordGraph.new(path)
  result.node.accept(graph)
  puts RecordExport.new(graph).emit
rescue ex
  STDERR.puts ex.message
  exit 1
end
