# Read-only adapter to actual Ruby pmod2lmod. Observes ornament discovery;
# all clipping, interning, table mapping and LNote semantics stay in Ruby.
require 'json'
require 'stringio'
root, input = ARGV.fetch(0), ARGV.fetch(1)
original_stdout = $stdout
$stdout = StringIO.new
parts = File.read(File.join(root,'autooisee.rb')).split(/^\$set = Setup\.new\s*$/,2)
raise 'cannot locate original main boundary' unless parts.size == 2
eval(parts.first, TOPLEVEL_BINDING, File.join(root,'autooisee.rb'))
class Object
  alias_method :actual_arrange_squize, :squize_ornament
def squize_ornament(base, orn)
  result = actual_arrange_squize(base, orn)
  $arrange_ornament_trace << result[1]
  result
end
end
result = JSON.parse(File.read(input)).fetch('cases').map do |c|
  cfg = c.fetch('config')
  settings = cfg.fetch('modes').each_with_index.map do |mode, i|
    h = {mode.chr => true}
    h['e'] = true if cfg.fetch('envelopes')[i] != 0
    h
  end
  $set = Struct.new(:orn_repeat,:max_offset,:chan_settings,:samples,:ornaments,
                    :cool_envelope,:envelope_changes_volume,:envelope_sample,:envelope_dsample).
    new(cfg.fetch('repeat'),cfg.fetch('offset'),settings,cfg.fetch('samples'),
        cfg.fetch('ornaments'),true,false,2,2)
  pmod = Array.new(3) { [] }
  c.fetch('groups').each do |g|
    chan = pmod.fetch(g.fetch('channel'))
    row = g.fetch('row')
    chan.fill([],chan.size...row+1) if chan.size <= row
    chan[row] = g.fetch('notes').map { |n| FNote.new(n[0],15,n[1]) }
  end
  $arrange_ornament_trace = []
  lmod, ornaments = pmod2lmod(pmod)
  lnotes = lmod.map do |chan|
    chan.each_with_index.map do |n,row|
      n.nil? ? nil : {row:row,note:{note:n.note,sample:n.sample,envelope:n.envelope,
        ornament:n.ornament,volume:n.volume,type:n.type,kind:n.kind,enote:n.enote}}
    end.compact
  end
  data = ornaments.sort_by { |text,id| id }.map { |text,id| text[1..-1].split(',').map(&:to_i) }
  {name:c.fetch('name'),lnotes:lnotes,ornaments:data,
   discovery_ids:$arrange_ornament_trace.map { |text| ornaments.fetch(text) }}
end
$stdout = original_stdout
puts JSON.generate(cases:result)
