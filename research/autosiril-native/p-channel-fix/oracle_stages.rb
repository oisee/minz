#!/usr/bin/env ruby
# Dump every original Ruby pipeline stage of the canonical Chronos invocation
# as JSON, so the native chain's per-stage wire files can be diffed against
# unchanged stage values. Read-only against /Users/alice/dev/autosiril; the
# original source is guarded by exact SHA256 (same pin as the checkpoint-3
# VT2 reproducer).
#
# Usage: ruby oracle_stages.rb [--autosiril /Users/alice/dev/autosiril]
require 'json'
require 'stringio'
require 'digest'

opts = {autosiril: '/Users/alice/dev/autosiril'}
args = ARGV.dup
while (k = args.shift)
  case k
  when '--autosiril' then opts[:autosiril] = args.shift
  else raise "unknown argument #{k}"
  end
end

root = File.realpath(opts[:autosiril])
source = File.read(File.join(root, 'autooisee.rb'))
raise 'original autooisee.rb differs' unless Digest::SHA256.hexdigest(source) ==
  '41f621b555fc089effd711319e723494576eb47ea0282847082ec91c1cdd4c01'
parts = source.split(/^\$set = Setup\.new\s*$/, 2)
raise 'original boundary missing' unless parts.size == 2
eval(parts.first, TOPLEVEL_BINDING, 'autooisee.rb')

CANONICAL_ARGV = ['chronos.mid', '2me,1p,1m', '8', '6', '12', '0', '64', '2', '6']

def fnote(j)
  j.nil? ? nil : {note: j.note, volume: (j.volume * 100).round, type: j.type}
end

out = {}
Dir.chdir(File.join(root, 'test')) do
  ARGV.replace(CANONICAL_ARGV)
  saved = $stdout
  $stdout = StringIO.new
  begin
    $set = Setup.new
    $set.load_sequence
    vmod = seq2vmod($set.sequence)
    rmod = vmod2rmod(vmod)
    key = detect_key(rmod)
    rmod = apply_diatonic_transpose(rmod, key)
    pmod = rmod2pmod(rmod)
    lmod, orns = pmod2lmod(pmod)
    dmod = apply_delays(lmod)
    mmod = downmix(dmod)
    out[:settings] = {max_row: $set.max_row, pattern_size: $set.pattern_size,
                      skip_lines: $set.skip_lines, per_beat: $set.per_beat,
                      per_delay: $set.per_delay, per_delay2: $set.per_delay2,
                      orn_repeat: $set.orn_repeat, max_offset: $set.max_offset,
                      real_key: $set.real_key, detect_key: key,
                      chan_settings: $set.chan_settings, sources: $set.sources}
    # Stage 2 boundary: the per-channel/row flattened pitch groups the arrange
    # stage consumes (pmod cells carry the full poly pitch set).
    out[:pmod] = pmod.map do |pchan|
      pchan.map { |pcell| pcell.nil? ? [] : pcell.map { |n| [n.note, n.type] } }
    end
    # Stage 3 boundary: LNote grid before delays, with interned ornaments.
    out[:lmod] = lmod.map do |lchan|
      lchan.each_index.map do |r|
        n = lchan[r]
        next if n.nil?
        {row: r, note: n.note, sample: n.sample, envelope: n.envelope,
         ornament: n.ornament, volume: (n.volume * 100).round,
         type: n.type, kind: n.kind,
         enote: n.enote.nil? ? -1 : n.enote}
      end.compact
    end
    out[:ornaments] = orns.keys.each_with_object({}) do |txt, h|
      h[orns[txt]] = txt
    end
    # Stage 4 boundary: delayed grid.
    out[:dmod] = dmod.map do |dchan|
      dchan.each_index.map do |r|
        n = dchan[r]
        next if n.nil?
        {row: r, note: n.note, sample: n.sample, envelope: n.envelope,
         ornament: n.ornament, volume: (n.volume * 100).round,
         type: n.type, kind: n.kind,
         enote: n.enote.nil? ? -1 : n.enote}
      end.compact
    end
    # Stage 5 boundary: mixed grid.
    out[:mmod] = mmod.map do |mchan|
      mchan.each_index.map do |r|
        n = mchan[r]
        next if n.nil?
        {row: r, note: n.note, sample: n.sample, envelope: n.envelope,
         ornament: n.ornament, volume: (n.volume * 100).round,
         type: n.type, kind: n.kind,
         enote: n.enote.nil? ? -1 : n.enote}
      end.compact
    end
  ensure
    $stdout = saved
  end
end
puts JSON.generate(out)
