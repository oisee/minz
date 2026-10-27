# Actual Ruby downmix and seq2vmod; the adapter restores verified wire fields.
require 'json'
require 'stringio'
root,input=ARGV.fetch(0),ARGV.fetch(1)
stdout=$stdout;$stdout=StringIO.new
parts=File.read(File.join(root,'autooisee.rb')).split(/^\$set = Setup\.new\s*$/,2)
raise 'original main boundary missing' unless parts.size==2
eval(parts.first,TOPLEVEL_BINDING,File.join(root,'autooisee.rb'))
def snapshot(channels)
  channels.map do |channel|
    channel.each_with_index.map do |n,row|
      n.nil? ? nil : {row:row,note:{note:n.note,sample:n.sample,envelope:n.envelope,ornament:n.ornament,volume:(n.volume*100).round,type:n.type,kind:n.kind,enote:n.enote}}
    end.compact
  end
end
ending_type=Struct.new(:time_from_start);event_type=Struct.new(:note,:time_from_start,:off)
result=JSON.parse(File.read(input)).fetch('cases').map do |c|
  cfg=c.fetch('config')
  raise 'invalid group counts' unless cfg.fetch('counts').size==3 && cfg.fetch('counts').sum==3
  groups=cfg.fetch('counts').map{|n|Array.new(n){999}} # Values are ignored by original downmix.
  $set=Struct.new(:sources_mix,:mix_options,:cool_envelope,:envelope_changes_volume,:envelope_sample,:envelope_dsample,:max_row,:cpr,:per_delay,:per_delay2).
    new(groups,cfg.fetch('policies'),true,false,2,2,0,1,c.fetch('first',6),c.fetch('second',12))
  tracks=Array.new(64){[]}
  c.fetch('paired_notes').each{|start,off,pitch,source|tracks.fetch(source)<<event_type.new(pitch,start,ending_type.new(off))}
  actual=seq2vmod(tracks).flatten.map{|n|[n.start,n.off,n.note]}
  expected=(0...64).flat_map{|s|c.fetch('paired_notes').select{|n|n[3]==s}.map{|n|n[0,3]}}
  raise 'paired row adapter changed notes' unless actual==expected
  channels=c.fetch('lengths').map{|n|Array.new(n)}
  c.fetch('channels').each_with_index do |ch,i|
    ch.each do |cell|
      n=cell.fetch('note');note=LNote.new(n.fetch('note'),n.fetch('sample'),n.fetch('envelope'),n.fetch('ornament'),n.fetch('volume')/100.0,n.fetch('type'),n.fetch('kind'))
      channels[i][cell.fetch('row')]=note
    end
  end
  raise 'adapter changed input fields' unless snapshot(channels)==c.fetch('channels').map{|ch|ch.map{|x|x.transform_keys(&:to_sym).merge(note:x.fetch('note').transform_keys(&:to_sym))}}
  before=snapshot(channels);mixed=downmix(channels)
  raise 'original downmix mutated input' unless snapshot(channels)==before
  {name:c.fetch('name'),channels:snapshot(mixed),lengths:mixed.map(&:size),render_rows:$set.max_row}
end
$stdout=stdout
puts JSON.generate(cases:result)
