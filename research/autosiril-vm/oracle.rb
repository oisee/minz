# Read-only adapter to actual Ruby Autosiril classes and pipeline functions.
# No converter output is written into its checkout.
require 'json'
require 'stringio'
require 'midilib'
root,path,beat,mapping=ARGV.fetch(0),ARGV.fetch(1),Integer(ARGV.fetch(2,'8')),ARGV.fetch(3,'2me,1p,1m')
original_stdout=$stdout
$stdout=StringIO.new
source=File.read(File.join(root,'autooisee.rb')).split(/^\$set = Setup\.new\s*$/,2).first
eval(source,TOPLEVEL_BINDING,File.join(root,'autooisee.rb'))
ARGV.replace([path,mapping,beat.to_s,'6','12','0','64','2','24'])
$set=Setup.new
$set.load_sequence
seq=$set.sequence
notes=[]
source_index=0
seq.tracks.each_with_index do |track,ti|
  paired=track.events.select{|e|e.respond_to?(:note)&&e.respond_to?(:off)&&e.off}
  next if paired.empty?
  paired.each do |e|
    v=VNote.new(e.note,e.time_from_start,e.off.time_from_start,15)
    notes << {track:ti,source:source_index,channel:e.channel,pitch:v.note,
              start_tick:e.time_from_start,off_tick:e.off.time_from_start,
              start:v.start,off:v.off}
  end
  source_index+=1
end
vmod=seq2vmod(seq)
rmod=vmod2rmod(vmod)
pmod=rmod2pmod(rmod)
flattened=pmod.map do |channel|
  channel.each_with_index.map{|cell,row|cell.empty? ? nil : {row:row,notes:cell.map{|n|[n.note,n.type]}}}.compact
end
$stdout=original_stdout
puts JSON.generate(per_beat:beat,sources:source_index,notes:notes,max_row:$set.max_row,flattened:flattened)
