# Independent wire-event oracle: midilib's parser callbacks before its sequence
# reader normalizes velocity-zero note-ons or synthesizes note-offs.
require 'json'
gem 'midilib', '= 4.0.2'
require 'midilib/io/midifile'
class WireReader < MIDI::IO::MIDIFile
  attr_reader :events, :file_header
  def initialize
    super
    @events = []
    @track = -1
    self.skip_init = false
  end
  def header(format, tracks, division)
    @file_header = [format, tracks, division]
  end
  def start_track
    @track += 1
  end
  def emit(status, a=0, b=0, meta=0, payload=[])
    @events << {track: @track, tick: @ticks_so_far, delta: @curr_ticks,
                status: status, data1: a, data2: b, meta: meta, payload: payload}
  end
  def chan_message(running, status, a, b)
    emit(status, a, b)
  end
  def meta_event(type)
    emit(255, 0, 0, type, msg.dup)
  end
  def handle_sysex(data)
    emit(240, 0, 0, 0, data[1..-1])
  end
  def handle_arbitrary(data)
    emit(247, 0, 0, 0, data.dup)
  end
end
reader = WireReader.new
File.open(ARGV.fetch(0), 'rb') { |io| reader.read_from(io) }
puts JSON.generate(header: reader.file_header, events: reader.events)
