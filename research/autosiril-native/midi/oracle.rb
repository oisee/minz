# Original Autosiril Setup/VNote and midilib pairing; no target algorithm copy.
require 'json'
require 'stringio'
require 'midilib'
root, path, beat = ARGV.fetch(0), ARGV.fetch(1), Integer(ARGV.fetch(2, '8'))
original_stdout = $stdout
$stdout = StringIO.new
parts = File.read(File.join(root, 'autooisee.rb')).split(/^\$set = Setup\.new\s*$/, 2)
raise 'reference main boundary changed' unless parts.size == 2
eval(parts.first, TOPLEVEL_BINDING, File.join(root, 'autooisee.rb'))
ARGV.replace([path, '1p', beat.to_s, '6', '12', '0', '64', '2', '24'])
$set = Setup.new
$set.load_sequence
notes = []
source = 0
$set.sequence.tracks.each do |track|
  paired = track.events.select { |event| event.respond_to?(:note) && event.respond_to?(:off) && event.off }
  next if paired.empty?
  paired.each do |event|
    note = VNote.new(event.note, event.time_from_start, event.off.time_from_start, 15)
    notes << {start: note.start, off: note.off, pitch: note.note, source: source}
  end
  source += 1
end
$stdout = original_stdout
puts JSON.generate(notes: notes, sources: source)
