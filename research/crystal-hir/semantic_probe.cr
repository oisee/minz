require "./native_frontend"

class ReachableProbe < Crystal::Visitor
  @seen = Set(UInt64).new

  def initialize(@path : String)
  end

  def visit(node : Crystal::ASTNode)
    true
  end

  def visit(node : Crystal::Call)
    node.target_defs.try &.each do |f|
      next unless f.location.try(&.filename) == @path
      next unless @seen.add?(f.object_id)
      puts "#{f.owner}##{f.name}(#{f.args.map { |a| "#{a.name}:#{a.type?}" }.join(", ")}) -> #{f.type?}"
      puts "  #{f.body.class}: #{f.body}"
      f.body.accept(self)
    end
    true
  end
end

path = File.expand_path(ARGV[0])
compiler = Crystal::Compiler.new
compiler.no_codegen = true
compiler.color = false
result = compiler.compile(Crystal::Compiler::Source.new(path, File.read(path)), "/tmp/crystal-unused")
result.node.accept(ReachableProbe.new(path))
