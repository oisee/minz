# Read-only paired-interval adapter to the actual Ruby Autosiril pipeline.
# cpr=1 preserves supplied quantized rows; no musical selection is reimplemented.
require 'json'
require 'stringio'
root, input = ARGV.fetch(0), ARGV.fetch(1)
original_stdout = $stdout
$stdout = StringIO.new
parts = File.read(File.join(root, 'autooisee.rb')).split(/^\$set = Setup\.new\s*$/, 2)
raise 'reference main boundary changed' unless parts.size == 2
eval(parts.first, TOPLEVEL_BINDING, File.join(root, 'autooisee.rb'))
cases = JSON.parse(File.read(input))
result = cases.map do |c|
  notes = c.fetch('notes')
  source = c.fetch('source')
  mode = c.fetch('mode')
  raise 'invalid oracle mode' unless %w[p m d].include?(mode)
  max_row = notes.map { |n| n.fetch('off') }.max || 0
  $set = Struct.new(:cpr, :sources, :max_row, :chan_settings).new(1, [source], max_row, [{mode => true}])
  source_count = [source + 1, (notes.map { |n| n.fetch('source') }.max || 0) + 1].max
  vmod = Array.new(source_count) { [] }
  notes.each do |n|
    vmod[n.fetch('source')] << VNote.new(n.fetch('pitch'), n.fetch('start'), n.fetch('off'), 15)
  end
  flattened = rmod2pmod(vmod2rmod(vmod)).fetch(0)
  events = flattened.each_with_index.flat_map do |cell, row|
    cell.map do |n|
      kind = {'s' => 1, 'r' => 4}.fetch(n.type)
      {row: row, value: n.note | (kind << 8)}
    end
  end
  {name: c.fetch('name'), events: events.take(c.fetch('limit')), full_count: events.size}
end
$stdout = original_stdout
puts JSON.generate(result)
