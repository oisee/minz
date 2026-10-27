# Reproducer for the 62-row divergence between the planning native_chronos
# render and the original full Ruby pipeline render of test/chronos.mid.
#
# Read-only with respect to /Users/alice/dev/autosiril. The original
# functions are eval'd from the unchanged source (guarded by exact SHA256)
# and run in-process with the canonical test/test_chronos.sh invocation.
# The planning fixture is only read.
#
# Usage:
#   ruby chronos_divergence_repro.rb \
#     [--autosiril /Users/alice/dev/autosiril] \
#     [--planning /tmp/minz-render-planning-v4]
#
# Output: JSON on stdout with (1) invocation settings as derived by the
# actual original Setup for the canonical ARGV, (2) the first differing
# rendered row and columns, (3) the cell-level diff between the kind5 wire
# input (cases.json entry "native_chronos", the native mix-stage output the
# native render stage consumed) and the actual Ruby mmod, and (4) the
# squize_ornament analysis of the first differing cell.
require 'json'
require 'stringio'
require 'digest'

defaults = {autosiril: '/Users/alice/dev/autosiril',
            planning: '/tmp/minz-render-planning-v4'}
args = ARGV.dup
opts = defaults.dup
while (k = args.shift)
  case k
  when '--autosiril' then opts[:autosiril] = args.shift
  when '--planning' then opts[:planning] = args.shift
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

rendered = nil
mmod = nil
pmod = nil
lmod_pre_delays = nil
settings = nil
Dir.chdir(File.join(root, 'test')) do
  ARGV.replace(CANONICAL_ARGV)
  saved = $stdout
  $stdout = StringIO.new  # the original steps print progress chatter
  begin
    $set = Setup.new
    $set.load_sequence
    vmod = seq2vmod($set.sequence)
    rmod = vmod2rmod(vmod)
    key = detect_key(rmod)
    rmod = apply_diatonic_transpose(rmod, key)
    pmod = rmod2pmod(rmod)
    lmod, ornaments = pmod2lmod(pmod)
    lmod_pre_delays = lmod
    lmod = apply_delays(lmod)
    mmod = downmix(lmod)
    rendered = render_into_text(mmod)
    settings = {max_row: $set.max_row, pattern_size: $set.pattern_size,
                skip_lines: $set.skip_lines, per_beat: $set.per_beat,
                per_delay: $set.per_delay, per_delay2: $set.per_delay2,
                orn_repeat: $set.orn_repeat, max_offset: $set.max_offset,
                real_key: $set.real_key, ppqn: $set.sequence.ppqn,
                cpr: $set.cpr, detect_key: key,
                chan_settings: $set.chan_settings, sources: $set.sources,
                ornaments: ornaments}
  ensure
    $stdout = saved
  end
end

planning = File.readlines(File.join(opts[:planning], 'native_chronos',
                                    'output.txt')).map(&:chomp)
kind5 = JSON.parse(File.read(File.join(opts[:planning], 'cases.json')))['cases'][0]
raise 'wrong planning case' unless kind5['name'] == 'native_chronos'

# --- (2) first differing rendered row -----------------------------------
diff_rows = (0...[rendered.size, planning.size].min)
             .select {|i| rendered[i] != planning[i] }
i = diff_rows.first
first_row = nil
if i
  a = rendered[i]
  b = planning[i]
  cols = (0...[a.size, b.size].min).select {|j| a[j] != b[j] }
  # line layout: "#{env}|..|noteA|noteB|noteC}"; noteB block is cols 22..35
  block_of = lambda {|c| c < 8 ? 'env' : c < 22 ? 'noteA' : c < 36 ? 'noteB' : 'noteC' }
  first_row = {row: i, original: a, planning: b, diff_cols: cols,
               blocks: cols.map {|c| block_of.call(c) }.uniq}
end

# --- (3) kind5 wire cells vs actual Ruby mmod ----------------------------
cell_diffs = []
kind5['channels'].each_with_index do |ch, ci|
  ch.each do |ev|
    r = ev['row']
    n = ev['note']
    m = mmod[ci][r]
    next if !m.nil? && n['note'] == m.note && n['ornament'] == m.ornament &&
            n['envelope'] == m.envelope && n['kind'] == m.kind
    cell_diffs << {channel: ci, row: r,
                   kind5: {note: n['note'], ornament: n['ornament'],
                           envelope: n['envelope'], kind: n['kind'],
                           volume: n['volume']},
                   ruby: m && {note: m.note, ornament: m.ornament,
                               envelope: m.envelope, kind: m.kind,
                               volume: m.volume}}
  end
end

# --- (4) squize_ornament analysis of the first differing cell ------------
# Ruby pmod cell -> base/ornament via squize_ornament (mediana filter with
# max_offset, then rebase by min offset); show what the actual Ruby pmod
# cell was and which ornament id the kind5 fixture's cell implies.
analysis = nil
if (d = cell_diffs.first)
  ci = d[:channel]
  r = d[:row]
  pcell = pmod[ci][r]
  base = pcell.min.note
  proto = pcell.uniq.sort.map {|fn| fn.note - base}.uniq
  # replay squize_ornament on a copy (it mutates its orn argument)
  mediana = proto.sort[proto.size / 2]
  kept = proto.reject {|off| (off - mediana).abs > settings[:max_offset] }
  min_off = kept.min
  analysis = {channel: ci, row: r,
              ruby_pmod_cell_notes: pcell.map(&:note),
              proto_ornament_offsets: proto,
              mediana: mediana, offsets_kept_after_max_offset_filter: kept,
              rebased_base_note: base + min_off,
              rebased_ornament_offsets: kept.map {|x| x - min_off },
              rebased_ornament: 'L' + kept.map {|x|
                [x - min_off] * settings[:orn_repeat] }.flatten.map(&:to_s).join(','),
              kind5_cell: d[:kind5], ruby_mmod_cell: d[:ruby]}
end

puts JSON.generate({
  original_source_sha256: Digest::SHA256.hexdigest(source),
  canonical_argv: CANONICAL_ARGV,
  settings_derived_by_original_setup: settings,
  planning: {rows: planning.size, kind5_render_rows: kind5['render_rows'],
             kind5_channels: kind5['channels'].map(&:size)},
  rendered: {rows: rendered.size},
  differing_rows: diff_rows.size,
  first_differing_row: first_row,
  differing_rows_list_head: diff_rows.first(20),
  kind5_vs_ruby_mmod: {differing_cells: cell_diffs.size,
                       by_channel: cell_diffs.group_by {|d| d[:channel]}
                                              .map {|k, v| [k, v.size] }.to_h,
                       first_cells: cell_diffs.first(5)},
  squize_analysis_first_cell: analysis
})
