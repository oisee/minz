# Read-only adapter to actual Ruby Autosiril classes and pipeline functions.
# No converter output is written into its checkout.
require 'json'
require 'stringio'
require 'midilib'
root,path,beat,mapping=ARGV.fetch(0),ARGV.fetch(1),Integer(ARGV.fetch(2,'8')),ARGV.fetch(3,'2me,1p,1m')
max_offset=ARGV.fetch(4,'24')
pattern_size=ARGV.fetch(5,'0')
skip_rows=ARGV.fetch(6,'64')
original_stdout=$stdout
$stdout=StringIO.new
parts=File.read(File.join(root,'autooisee.rb')).split(/^\$set = Setup\.new\s*$/,2)
raise 'cannot locate reference main boundary' unless parts.size==2
source=parts.first
eval(source,TOPLEVEL_BINDING,File.join(root,'autooisee.rb'))
ARGV.replace([path,mapping,beat.to_s,'6','12',pattern_size,skip_rows,'2',max_offset])
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
lmod,ornaments=pmod2lmod(pmod)
lnotes=lmod.map{|channel|channel.each_with_index.map{|n,row|n.nil? ? nil : {row:row,note:{note:n.note,sample:n.sample,envelope:n.envelope,ornament:n.ornament,volume:n.volume,type:n.type,kind:n.kind,enote:n.enote}}}.compact}
ornament_data=ornaments.sort_by{|text,id|id}.map{|text,id|text[1..-1].split(',').map(&:to_i)}
dump_lmod=lambda{|mod|mod.map{|channel|channel.each_with_index.map{|n,row|n.nil? ? nil : {row:row,note:{note:n.note,sample:n.sample,envelope:n.envelope,ornament:n.ornament,volume:n.volume,type:n.type,kind:n.kind,enote:n.enote}}}.compact}}
delayed=dump_lmod.call(apply_delays(lmod))
mixed_mod=downmix(apply_delays(lmod))
mixed=dump_lmod.call(mixed_mod)
text_lines=render_into_text(mixed_mod)
rows_text=text_lines.empty? ? '' : text_lines.join("\n")+"\n"
patterns,used,play_order=split_into_patterns(text_lines)
ornaments_text=render_ornaments(ornaments)
require File.join(root,'module_template.rb')
vt_text=module_header(mapping,play_order,ornaments_text)+"\n"
patterns.each_with_index do |text,i|
  vt_text+="\n[Pattern#{i}]\n"+text if used[i] && text!=''
end
$stdout=original_stdout
puts JSON.generate(per_beat:beat,sources:source_index,notes:notes,max_row:$set.max_row,flattened:flattened,lnotes:lnotes,ornaments:ornament_data,delayed:delayed,mixed:mixed,rows_text:rows_text,patterns:patterns,pattern_used:patterns.each_index.map{|i|used[i]},play_order:play_order,vt_text:vt_text)
