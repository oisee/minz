# Freeze only data constants from the user's reference, no main/file writes.
require 'json'
root=ARGV.fetch(0)
parts=File.read(File.join(root,'autooisee.rb')).split(/^\$set = Setup\.new\s*$/,2)
raise 'cannot locate reference main boundary' unless parts.size==2
eval(parts.first,TOPLEVEL_BINDING,File.join(root,'autooisee.rb'))
puts JSON.generate(env_offsets:ENV_OFFSETS,env_forms:ENV_FORMS,drum_samples:NOTE2DRUM_SAMPLE,drum_notes:NOTE2DRUM_NOTE)
