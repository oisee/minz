require 'json'
require 'stringio'
require 'digest'
root,input_path,out=ARGV
source=File.read(File.join(root,'autooisee.rb'))
raise 'source differs' unless Digest::SHA256.hexdigest(source)=='41f621b555fc089effd711319e723494576eb47ea0282847082ec91c1cdd4c01'
prefix=source.split(/^\$set = Setup\.new\s*$/,2);raise 'missing boundary' unless prefix.size==2
eval(prefix.first,TOPLEVEL_BINDING,File.join(root,'autooisee.rb'))
c=JSON.parse(File.read(input_path))
$set=Struct.new(:cool_envelope,:envelope_changes_volume,:envelope_sample,:envelope_dsample,:max_row).new(true,false,2,2,c.fetch('render_rows'))
channels=c.fetch('lengths').map{|n|Array.new(n)}
c.fetch('channels').each_with_index{|ch,i|ch.each{|cell|
 n=cell.fetch('note');x=LNote.new(n['note'],n['sample'],n['envelope'],n['ornament'],n['volume']/100.0,n['type'],n['kind'])
 actual={note:x.note,sample:x.sample,envelope:x.envelope,ornament:x.ornament,volume:(x.volume*100).round,type:x.type,kind:x.kind,enote:x.enote}
 raise 'adapter changed fields' unless actual.transform_keys(&:to_s)==n
 channels[i][cell.fetch('row')]=x
}}
saved=$stdout;$stdout=StringIO.new
lines=render_into_text(channels)
$stdout=saved
raise 'wrong row count' unless lines.size==c['render_rows']
bytes=lines.join("\n")+(lines.empty? ? '' : "\n")
File.binwrite(File.join(out,'chronos.txt'),bytes)
puts JSON.generate(scope:'actual Ruby render on saved native MIXED, no target rendering proof',rows:lines.size,line_widths:lines.map(&:bytesize).uniq,bytes:bytes.bytesize,sha256:Digest::SHA256.hexdigest(bytes),input_sha256:c['input_sha256'],first:lines.take(3),float_param_checks:[7.35,10.5,15.0].map{|v|[v,PARAMS[v]]})
