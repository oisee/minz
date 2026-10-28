# Actual Ruby renderer with a read-only TracePoint at the outer row block return.
require 'json'
require 'stringio'
require 'digest'
root, input, out = ARGV
file = File.join(root, 'autooisee.rb')
source = File.read(file)
raise 'original Ruby source differs' unless Digest::SHA256.hexdigest(source) == '41f621b555fc089effd711319e723494576eb47ea0282847082ec91c1cdd4c01'
parts = source.split(/^\$set = Setup\.new\s*$/, 2)
raise 'original boundary missing' unless parts.size == 2
raise 'trace location differs' unless source.lines[979].strip == 'pforms = cforms' && source.lines[980].strip == '}'
eval(parts.first, TOPLEVEL_BINDING, file)

def wire(n)
  {note:n.note, sample:n.sample, envelope:n.envelope, ornament:n.ornament,
   volume:(n.volume*100).round, type:n.type, kind:n.kind, enote:n.enote}
end

def state(n)
  [n.note, n.sample, n.envelope, n.ornament, (n.volume*100).round,
   n.type, n.kind, n.enote, n.pitch, n.oct, n.epitch, n.eoct,
   n.instance_variable_get(:@pnote)]
end

result = JSON.parse(File.read(input)).fetch('cases').map do |c|
  name = c.fetch('name')
  raise 'invalid case name' unless /\A[a-zA-Z0-9_+-]+\z/.match?(name)
  $set = Struct.new(:cool_envelope,:envelope_changes_volume,:envelope_sample,
                    :envelope_dsample,:max_row).new(true,false,2,2,c.fetch('render_rows'))
  channels = if c.key?('raw_channels')
    c.fetch('raw_channels').map do |ch|
      length = ch.empty? ? 0 : ch.map{|a|a[0]}.max+1
      cells = Array.new(length)
      ch.each do |r,n,s,e,o,v,t,k|
        raise 'duplicate raw row' unless cells[r].nil?
        cells[r] = LNote.new(n,s,e,o,v/100.0,t,k)
      end
      cells
    end
  else
    c.fetch('channels').each_with_index.map do |ch,i|
      cells = Array.new(c.fetch('lengths')[i])
      ch.each do |cell|
        n = cell.fetch('note')
        x = LNote.new(n['note'],n['sample'],n['envelope'],n['ornament'],n['volume']/100.0,n['type'],n['kind'])
        raise 'adapter changed source fields' unless wire(x).transform_keys(&:to_s) == n
        cells[cell.fetch('row')] = x
      end
      cells
    end
  end
  initial = {name:name, render_rows:c.fetch('render_rows'), lengths:channels.map(&:size),
             channels:channels.map{|ch|ch.each_with_index.map{|n,r|n.nil? ? nil : {row:r,note:wire(n)}}.compact}}
  states = []
  trace = TracePoint.new(:b_return) do |tp|
    if tp.path == file && tp.method_id == :render_into_text && tp.lineno == 981
      b = tp.binding
      states << [b.local_variable_get(:row_i), b.local_variable_get(:pforms).clone,
                 state(b.local_variable_get(:penote)), b.local_variable_get(:cnotes).map{|n|state(n)}]
    end
  end
  saved = $stdout
  $stdout = StringIO.new
  begin
    lines = trace.enable { render_into_text(channels) }
  ensure
    trace.disable
    $stdout = saved
  end
  raise 'missing row observations' unless states.size == lines.size && lines.size == c.fetch('render_rows')
  directory = File.join(out,name)
  Dir.mkdir(directory)
  bytes = lines.join("\n") + (lines.empty? ? '' : "\n")
  File.binwrite(File.join(directory,'output.txt'),bytes)
  File.write(File.join(directory,'states.json'),JSON.generate(states))
  {name:name, initial:initial, rows:lines.size, bytes:bytes.bytesize,
   output_sha256:Digest::SHA256.hexdigest(bytes), widths:lines.map(&:bytesize).uniq.sort}
end
puts JSON.generate(cases:result)
