# Actual Ruby Step9/10/11 (split_into_patterns, render_ornaments,
# write_vt_output) with a pinned clock. The original functions are eval'd
# from the unchanged original source and called directly; nothing here
# reimplements them. Read-only with respect to /Users/alice/dev/autosiril.
require 'json'
require 'stringio'
require 'digest'
require 'fileutils'
script_root, script_input, script_out = ARGV
root = File.realpath(script_root)
file = File.join(root, 'autooisee.rb')
template_file = File.join(root, 'module_template.rb')
source = File.read(file)
raise 'original autooisee.rb differs' unless Digest::SHA256.hexdigest(source) == '41f621b555fc089effd711319e723494576eb47ea0282847082ec91c1cdd4c01'
raise 'original module_template.rb differs' unless Digest::SHA256.hexdigest(File.read(template_file)) == '8037f85a1c92cc32e494ba6b62a70be99fae5de041050addbf05844b7e254ad6'
parts = source.split(/^\$set = Setup\.new\s*$/, 2)
raise 'original boundary missing' unless parts.size == 2
raise 'split_into_patterns moved' unless source.lines[985].strip == 'def split_into_patterns(text_lines)'
raise 'render_ornaments moved' unless source.lines[1036].strip == 'def render_ornaments(ornaments)'
raise 'write_vt_output moved' unless source.lines[1051].strip == 'def write_vt_output(wet_patterns, use_patterns, wet_play_order_txt, ornaments_txt)'
eval(parts.first, TOPLEVEL_BINDING, file)

# module_header embeds Time.new.strftime("%Y.%m.%d"). The date is genuinely
# environment-derived; for a byte-reproducible proof both this oracle and the
# independent model use the same pinned instant. Recorded in the receipt.
PINNED = Time.at(0).utc
raise 'pinned clock unexpected' unless PINNED.strftime('%Y.%m.%d') == '1970.01.01'
Time.singleton_class.send(:define_method, :new) do |*args|
  PINNED
end

result = JSON.parse(File.read(script_input)).fetch('cases').map do |c|
  name = c.fetch('name')
  raise 'invalid case name' unless /\A[a-zA-Z0-9_+-]+\z/.match?(name)
  raise 'pattern_size must be integer' unless c.fetch('pattern_size').is_a?(Integer)
  raise 'skip_lines must be integer' unless c.fetch('skip_lines').is_a?(Integer)
  # $set carries only what Step9 actually reads; ARGV carries exactly the
  # caller-supplied argv (title semantics of ARGV[1] and ArgList come from
  # these values, never from this oracle script's own argv).
  $set = Struct.new(:max_row, :pattern_size, :skip_lines, :out_file).new(
    c.fetch('max_row'), c.fetch('pattern_size'), c.fetch('skip_lines'), nil)
  out_path = File.join(script_out, name + '.vt2')
  FileUtils.mkdir_p(script_out)
  $set.out_file = out_path
  ARGV.replace(c.fetch('argv').map(&:to_s))
  lines = c.fetch('lines')
  ornaments = c.fetch('ornaments')
  saved = $stdout
  $stdout = StringIO.new
  begin
    wet_patterns, use_patterns, wet_play_order_txt = split_into_patterns(lines)
    ornaments_txt = render_ornaments(ornaments)
    write_vt_output(wet_patterns, use_patterns, wet_play_order_txt, ornaments_txt)
  ensure
    $stdout = saved
  end
  # The original write_vt_output never closes txt_file; its buffered bytes
  # must be flushed (GC finalizer) before the file is hashed. Oracle-side
  # plumbing only; the original functions themselves are unchanged.
  GC.start
  raise 'pattern bookkeeping mismatch' unless wet_patterns.size == use_patterns.size
  {name: name, patterns: wet_patterns.size,
   use_patterns: (0...wet_patterns.size).map {|i| use_patterns[i]},
   play_order: wet_play_order_txt,
   wet_sizes: wet_patterns.map(&:bytesize),
   ornaments_txt_sha256: Digest::SHA256.hexdigest(ornaments_txt),
   ornaments_txt_bytes: ornaments_txt.bytesize,
   file_bytes: File.size(out_path),
   file_sha256: Digest::SHA256.hexdigest(File.binread(out_path))}
end
puts JSON.generate(cases: result)
