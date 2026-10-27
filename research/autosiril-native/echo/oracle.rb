require 'json'
require 'stringio'
root,path=ARGV.fetch(0),ARGV.fetch(1)
stdout=$stdout;$stdout=StringIO.new
parts=File.read(File.join(root,'autooisee.rb')).split(/^\$set = Setup\.new\s*$/,2)
raise 'main boundary missing' unless parts.size==2
eval(parts.first,TOPLEVEL_BINDING,File.join(root,'autooisee.rb'))
ending_type=Struct.new(:time_from_start)
event_type=Struct.new(:note,:time_from_start,:off)
result=JSON.parse(File.read(path)).fetch('cases').map do |c|
  cfg=c.fetch('config')
  settings=3.times.map do |i|
    h={};h['u']=true if cfg.fetch('unvoiced')[i];h['w']=true if cfg.fetch('wide')[i];h
  end
  $set=Struct.new(:chan_settings,:per_delay,:per_delay2,:cool_envelope,:envelope_changes_volume,:envelope_sample,:envelope_dsample,:max_row,:cpr).
    new(settings,cfg.fetch('first'),cfg.fetch('second'),true,false,2,2,0,1)
  tracks=Array.new(64){[]}
  c.fetch('paired_notes').each do |start,off,pitch,source|
    ending=ending_type.new(off)
    tracks.fetch(source) << event_type.new(pitch,start,ending)
  end
  actual_notes=seq2vmod(tracks).flatten
  observed=actual_notes.map{|n|[n.start,n.off,n.note]}
  expected=(0...64).flat_map{|source|c.fetch('paired_notes').select{|n|n[3]==source}.map{|n|n[0,3]}}
  raise "adapter changed paired rows #{c['name']} #{observed.take(3)} vs #{expected.take(3)} count#{observed.size}/#{expected.size}" unless observed==expected
  base_rows=$set.max_row+1
  raise 'independent Ruby extent differs' unless base_rows==c.fetch('base_rows')
  channels=Array.new(3){Array.new(base_rows)}
  c.fetch('channels').each_with_index do |ch,i|
    ch.each do |cell|
      n=cell.fetch('note')
      note=LNote.new(n.fetch('note'),n.fetch('sample'),n.fetch('envelope'),n.fetch('ornament'),n.fetch('volume')/100.0,n.fetch('type'),n.fetch('kind'))
      actual={note:note.note,sample:note.sample,envelope:note.envelope,ornament:note.ornament,volume:(note.volume*100).round,type:note.type,kind:note.kind,enote:note.enote}
      raise 'adapter changed initial LNote' unless actual.transform_keys(&:to_s)==n
      channels[i][cell.fetch('row')]=note
    end
  end
  delayed=apply_delays(channels)
  snapshots=delayed.map do |ch|
    ch.each_with_index.map do |n,row|
      n.nil? ? nil : {row:row,note:{note:n.note,sample:n.sample,envelope:n.envelope,ornament:n.ornament,volume:(n.volume*100).round,type:n.type,kind:n.kind,enote:n.enote}}
    end.compact
  end
  {name:c.fetch('name'),channels:snapshots,lengths:delayed.map(&:size),base_rows:base_rows}
end
$stdout=stdout
puts JSON.generate(cases:result)
