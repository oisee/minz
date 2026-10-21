#!/usr/bin/env python3
"""Seeded differential Z80 fuzzer. See scripts/README.md."""
import random, os, sys
S = os.path.dirname(os.path.abspath(__file__))
M = 0xFFFF

class Gen:
    def __init__(s, seed):
        s.r = random.Random(seed)
        s.helpers = []  # (name, nparams, body_src, pyfunc)

    def expr16(s, vs, v8, d, calls=True):
        r = s.r
        if d <= 0 or r.random() < 0.3:
            c = r.random()
            if c < 0.75 and vs: return r.choice(vs)
            if c < 0.85 and v8: return f"({r.choice(v8)} as u16)"
            return str(r.choice([0, 1, 2, 3, 7, 100, 255, 256, 1000, 32768, 65535, r.randrange(65536)]))
        c = r.random()
        if c < 0.55:
            op = r.choice(['+', '-', '-', '&', '|', 'xor', '+', '-'])
            return f"({s.expr16(vs, v8, d-1, calls)} {op} {s.expr16(vs, v8, d-1, calls)})"
        if c < 0.70:
            op = r.choice(['/', '%'])
            k = r.random()
            if k < 0.5:
                return f"({s.expr16(vs, v8, d-1, calls)} {op} {r.choice([1,2,4,8,16,3,7,10,256])})"
            return f"({s.expr16(vs, v8, d-1, calls)} {op} ({s.expr16(vs, v8, d-1, calls)} | 1))"
        if c < 0.85 and calls and s.helpers:
            h = r.choice(s.helpers)
            args = ', '.join(s.expr16(vs, v8, d-1, calls) for _ in range(h[1]))
            return f"{h[0]}({args})"
        return f"({s.expr16(vs, v8, d-1, calls)} + {s.expr16(vs, v8, d-1, calls)})"

    def expr8(s, v8, vs, d):
        r = s.r
        if d <= 0 or r.random() < 0.4:
            if v8 and r.random() < 0.7: return r.choice(v8)
            if vs and r.random() < 0.5: return f"({r.choice(vs)} as u8)"
            return str(r.randrange(256))
        op = r.choice(['+', '-', '&', '|', 'xor'])
        return f"({s.expr8(v8, vs, d-1)} {op} {s.expr8(v8, vs, d-1)})"

    def cmp(s, vs):
        a, b = s.r.sample(vs, 2) if len(vs) > 1 else (vs[0], vs[0])
        return f"{a} {s.r.choice(['<', '>', '==', '!=', '<=', '>='])} {b}"

    def block(s, vs, v8, mut, depth, n, calls=True):
        r = s.r; out = []
        for _ in range(n):
            c = r.random()
            if c < 0.35:
                nm = f"t{s.cnt}"; s.cnt += 1
                out.append(f"let {nm}: u16 = {s.expr16(vs, v8, 2, calls)}"); vs = vs + [nm]
            elif c < 0.45:
                nm = f"b{s.cnt}"; s.cnt += 1
                out.append(f"let {nm}: u8 = {s.expr8(v8, vs, 2)}"); v8 = v8 + [nm]
            elif c < 0.65 and mut:
                m = r.choice(mut)
                out.append(f"{m} = {s.expr16(vs, v8, 2, calls)}")
            elif c < 0.85 and depth > 0 and mut:
                th, _, _ = s.block(vs, v8, mut, depth-1, r.randint(1, 2), calls)
                el, _, _ = s.block(vs, v8, mut, depth-1, r.randint(0, 2), calls)
                st = f"if {s.cmp(vs)} {{\n" + '\n'.join(th) + "\n}"
                if el: st += " else {\n" + '\n'.join(el) + "\n}"
                out.append(st)
            elif depth > 0 and mut:
                iv = f"lp{s.cnt}"; s.cnt += 1
                k = r.randint(0, 4)
                body, _, _ = s.block(vs + [iv], v8, mut, depth-1, r.randint(1, 2), calls)
                out.append(f"var {iv}: u16 = 0\nwhile {iv} < {k} {{\n" + '\n'.join(body) + f"\n{iv} = ({iv} + 1)\n}}")
        return out, vs, v8

    def program(s):
        r = s.r; s.cnt = 0; src = []
        for h in range(r.randint(1, 3)):
            np_ = r.randint(1, 3)
            ps = [f"p{k}" for k in range(np_)]
            body, vs, v8 = s.block(ps, [], [], 0, r.randint(1, 3), calls=False)
            name = f"h{h}"
            ret = s.expr16(vs, v8, 2, calls=False)
            src.append(f"fun {name}({', '.join(p + ': u16' for p in ps)}) -> u16 {{\n" + '\n'.join(body) + f"\nreturn ({ret})\n}}")
            s.helpers.append((name, np_))
        params = ['a', 'b', 'c']
        mut = ['v0', 'v1']
        pre = ["let d: u8 = ((a as u8) xor (c as u8))", "var v0: u16 = a", "var v1: u16 = (b xor c)"]
        body, vs, v8 = s.block(params + mut, ['d'], mut, 2, r.randint(4, 8))
        allv = vs + [f"({x} as u16)" for x in v8]
        r.shuffle(allv)
        ret = ' + '.join(f"({x} xor {i*37+1})" for i, x in enumerate(allv[:8]))
        src.append("fun f(a: u16, b: u16, c: u16) -> u16 {\n" + '\n'.join(pre + body) + f"\nreturn ({ret})\n}}")
        return '\n'.join(src)

class MaskedLoopGen(Gen):
    """Opt-in byte predicates with a live accumulator and word loop state."""
    def program(s):
        mask = s.r.choice([1, 2, 128, 15])
        predicate = s.r.choice(['!= 0', '== 0', f'== {mask}'])
        return f"""fun f(a: u16, b: u16, c: u16) -> u16 {{
var weight: u16 = a
var x: u8 = (b as u8)
var result: u16 = c
while x != 0 {{
if (x & {mask}) {predicate} {{
result = (result + weight)
}}
weight = (weight + weight)
x = (x / 2)
}}
return result
}}"""


class SignedGen(Gen):
    """Opt-in signed corpus; the original Gen consumes exactly the same RNG."""
    def program(s):
        r = s.r
        out = ["fun signed_return(x: i8) -> i16 {", "return x", "}",
               "fun signed_identity(x: i16) -> i16 {", "return x", "}",
               "fun signed_relay(x: i8) -> i16 {", "return signed_identity(x)", "}",
               "fun f(a: u16, b: u16, c: u16) -> u16 {",
               "var x: i16 = (a as i16)", "var y: i16 = (b as i16)",
               "var z: i8 = (c as i8)", "var w: i16 = z", "w = z",
               "x = (x + signed_return(z))", "y = (y + signed_relay(z))"]
        for left, right in [('z', 'x'), ('y', 'z')]:
            cmp = r.choice(['<', '<=', '>', '>=', '==', '!='])
            out += [f"if {left} {cmp} {right} {{", "w = signed_identity(z)",
                    "} else {", "w = (w + -1)", "}"]
        for _ in range(r.randint(3, 7)):
            ty, name = r.choice([('i16', 'x'), ('i16', 'y'), ('i8', 'z')])
            k = r.choice([-1, -2, -3, -7, -127, -128] if ty == 'i8' else
                         [-1, -2, -3, -7, -128, -129, -255, -256, -32768])
            op = r.choice(['+', '-', '*', '/', '%'])
            cmp = r.choice(['<', '<=', '>', '>=', '==', '!='])
            peer = r.choice(['x', 'y', 'z']) if ty == 'i16' else 'z'
            out += [f"if {name} {cmp} {k} {{", f"{name} = ({name} {op} {k})",
                    "} else {", f"{name} = ({peer} + {k})", "}"]
        out += ["return ((((x as u16) xor (y as u16)) + (z as u16)) xor (w as u16))", "}"]
        return '\n'.join(out)


def wrap(value, ty):
    width = 8 if ty.endswith('8') else 16
    value &= (1 << width) - 1
    return value - (1 << width) if ty.startswith('i') and value >= 1 << (width - 1) else value


# --- tiny interpreter for the generated subset ---
import re
def tokenize(t): return re.findall(r'[A-Za-z_]\w*|\d+|==|!=|<=|>=|[-+*/%&|(){}<>,:=]', t)

class Interp:
    def __init__(s, src):
        s.funcs = {}
        for m in re.finditer(r'fun (\w+)\(([^)]*)\) -> u16 \{\n(.*?)\n\}(?=\nfun |\Z)', src, re.S):
            ps = [(p.split(':')[0].strip(), p.split(':')[1].strip()) for p in m.group(2).split(',') if p.strip()]
            s.funcs[m.group(1)] = (ps, m.group(3).split('\n'))
        s.steps = 0
    def call(s, name, args):
        ps, lines = s.funcs[name]
        env = {}; ty = {}
        for (p, t), a in zip(ps, args):
            env[p] = wrap(a, t); ty[p] = t
        r = s.run(lines, 0, len(lines), env, ty)
        return r[1]
    def match(s, lines, i):
        depth = 1
        for j in range(i+1, len(lines)):
            l = lines[j].strip()
            if l.startswith('}'):
                depth -= 1
                if depth == 0: return j
            if l.endswith('{'): depth += 1
        raise Exception('unbalanced')
    def run(s, lines, i, end, env, ty):
        while i < end:
            l = lines[i].strip(); s.steps += 1
            if not l: i += 1; continue
            if s.steps > 200000: raise Exception('steps')
            m = re.match(r'(let|var) (\w+): ([ui]8|[ui]16) = (.*)$', l)
            if m:
                t = m.group(3); env[m.group(2)] = wrap(s.ev(m.group(4), env, ty), t); ty[m.group(2)] = t; i += 1; continue
            m = re.match(r'return (.*)$', l)
            if m: return ('ret', s.ev(m.group(1), env, ty) & M)
            m = re.match(r'(if|while) (.*) \{$', l)
            if m:
                j = s.match(lines, i)
                if m.group(1) == 'while':
                    while s.ev(m.group(2), env, ty):
                        r = s.run(lines, i+1, j, env, ty)
                        if r: return r
                    i = j + 1; continue
                # if: lines[j] is '}' or '} else {'
                if lines[j].strip() == '} else {':
                    k = s.match(lines, j)
                    if s.ev(m.group(2), env, ty): r = s.run(lines, i+1, j, env, ty)
                    else: r = s.run(lines, j+1, k, env, ty)
                    if r: return r
                    i = k + 1; continue
                if s.ev(m.group(2), env, ty):
                    r = s.run(lines, i+1, j, env, ty)
                    if r: return r
                i = j + 1; continue
            m = re.match(r'(\w+) = (.*)$', l)
            if m:
                t = ty[m.group(1)]; env[m.group(1)] = wrap(s.ev(m.group(2), env, ty), t); i += 1; continue
            raise Exception('bad line ' + l)
    def ev(s, e, env, ty):
        toks = tokenize(e); pos = [0]
        def peek(): return toks[pos[0]] if pos[0] < len(toks) else None
        def nxt(): t = toks[pos[0]]; pos[0] += 1; return t
        def prim():
            t = nxt()
            if t == '(':
                v, w = full(); assert nxt() == ')'; return v, w
            # Nanz literals have the smallest unsigned type that fits. An
            # enclosing u16 binding does not widen a nested byte operation.
            if t.isdigit(): return int(t), (8 if int(t) <= 255 else 16)
            if peek() == '(' and t in s.funcs:
                nxt(); args = []
                while True:
                    v, w = full(); args.append(v)
                    if nxt() == ')': break
                return s.call(t, args), 16
            return env[t], (8 if ty[t] == 'u8' else 16)
        def cast(v, w):
            if peek() == 'as':
                nxt(); t = nxt()
                return (v & 0xFF, 8) if t == 'u8' else (v & M, 16)
            return v, w
        def full():
            v, w = cast(*prim())
            while peek() in ('+', '-', '&', '|', 'xor', '/', '%'):
                op = nxt(); v2, w2 = cast(*prim())
                # Primitive unsigned operations use the widest operand.
                wd = max(w, w2)
                mask = 0xFF if wd == 8 else M
                if op == '+': v = (v + v2) & mask
                elif op == '-': v = (v - v2) & mask
                elif op == '&': v = v & v2
                elif op == '|': v = v | v2
                elif op == 'xor': v = v ^ v2
                elif op == '/': v = v // v2
                elif op == '%': v = v % v2
                v &= mask; w = wd
            if peek() in ('<', '>', '==', '!=', '<=', '>='):
                op = nxt(); v2, _ = cast(*prim())
                v = int({'<': v < v2, '>': v > v2, '==': v == v2, '!=': v != v2, '<=': v <= v2, '>=': v >= v2}[op]); w = 16
            return v, w
        v, w = full()
        return v


class SignedInterp(Interp):
    """Typed two's-complement oracle; quotient truncates toward zero as OpSDiv.

    Remainder is a - trunc(a/b)*b (OpSMod), rather than Python's floor modulo.
    Keep signed values at each operation width, including byte overflow.
    """
    def __init__(s, src):
        s.funcs = {}
        s.return_types = {}
        for m in re.finditer(r'fun (\w+)\(([^)]*)\) -> ([ui](?:8|16)) \{\n(.*?)\n\}(?=\nfun |\Z)', src, re.S):
            ps = [(p.split(':')[0].strip(), p.split(':')[1].strip())
                  for p in m.group(2).split(',') if p.strip()]
            s.funcs[m.group(1)] = (ps, m.group(4).split('\n'))
            s.return_types[m.group(1)] = m.group(3)
        s.steps = 0

    def call(s, name, args):
        return wrap(super().call(name, args), s.return_types[name])

    def ev(s, e, env, ty):
        toks = tokenize(e)
        pos = 0
        def peek(): return toks[pos] if pos < len(toks) else None
        def take():
            nonlocal pos
            t = toks[pos]; pos += 1; return t
        def width(t): return 8 if t.endswith('8') else 16
        def common(t, u):
            if 'u16' in (t, u): return 'u16'
            return u if width(u) > width(t) else t
        def prim():
            t = take()
            if t == '-':
                v, _ = prim(); v = -v
                return v, 'i8' if -128 <= v <= 127 else 'i16'
            if t == '(':
                v, t = full(); assert take() == ')'; return v, t
            if t.isdigit(): return int(t), 'u8' if int(t) <= 255 else 'u16'
            if peek() == '(' and t in s.funcs:
                take(); args = []
                while peek() != ')':
                    v, _ = full(); args.append(v)
                    if peek() != ',': break
                    take()
                assert take() == ')'
                return s.call(t, args), s.return_types[t]
            return env[t], ty[t]
        def cast():
            v, t = prim()
            while peek() == 'as':
                take(); t = take(); v = wrap(v, t)
            return v, t
        def full():
            v, t = cast()
            while peek() in ('+', '-', '*', '/', '%', '&', '|', 'xor'):
                op = take(); u, ut = cast(); t = common(t, ut)
                v, u = wrap(v, t), wrap(u, t)
                if op == '+': v += u
                elif op == '-': v -= u
                elif op == '*': v *= u
                elif op == '&': v &= u
                elif op == '|': v |= u
                elif op == 'xor': v ^= u
                else:
                    if u == 0: raise ZeroDivisionError()
                    q = (abs(v) // abs(u)) * (-1 if (v < 0) != (u < 0) else 1)
                    v = q if op == '/' else v - q * u
                v = wrap(v, t)
            if peek() in ('<', '>', '==', '!=', '<=', '>='):
                op = take(); u, ut = cast()
                ct = ut if width(ut) > width(t) else t
                v, u = wrap(v, ct), wrap(u, ct)
                v = int({'<': v < u, '>': v > u, '==': v == u, '!=': v != u,
                         '<=': v <= u, '>=': v >= u}[op]); t = 'u8'
            return v, t
        v, _ = full()
        assert pos == len(toks), (e, toks[pos:])
        return v


import argparse
import concurrent.futures
import json
from pathlib import Path
import tempfile
from assert_matrix import run_compiler, positive


def generated(seed, signed=False, masked_loops=False):
    gen = MaskedLoopGen if masked_loops else SignedGen if signed else Gen
    src = gen(seed).program()
    rr = random.Random(seed * 7 + 1)
    args = [rr.randrange(65536) if rr.random() < .6 else rr.randrange(300) for _ in range(3)]
    return src, args


def with_assert(src, args, mode="folded"):
    """Direct calls keep f parameterized when Z80 executes the assertion."""
    val = (SignedInterp(src) if re.search(r'\bi(?:8|16)\b', src) else Interp(src)).call('f', args)
    # Keep the existing wrapper as an emission root in both modes; only the
    # assertion target changes, so direct-call executes f rather than folded g.
    wrapper = f'\nfun g() -> u16 {{ return f({args[0]}, {args[1]}, {args[2]}) }}\n'
    target = f'f({args[0]}, {args[1]}, {args[2]})' if mode == 'direct-call' else 'g()'
    program = src + wrapper + f'assert {target} == {val} via z80\n'
    # The assembler treats F as the flags register, making CALL f invalid.
    # Use an unambiguous symbol so direct mode reaches the function body.
    return re.sub(r'\bf(?=\()', 'fuzz_entry', program) if mode == 'direct-call' else program


def mismatch(result):
    # Syntax/codegen errors and timeouts are reported separately, never used
    # as evidence that a reduced program still reproduces a wrong value.
    if 'exit_code' in result and (result['exit_code'] != 1 or
                                  result.get('executed') != 1 or
                                  result.get('passed') != 0 or result.get('failed') != 1):
        return False
    return not result['pass'] and bool(re.search(r'(?i)\bgot\s+-?(?:0x[0-9a-f]+|\d+)[,\s]+(?:want|expected)\b', result['error']))


def minimize(src, args, check, mode="folded"):
    """Greedy deletion: 1-minimal over complete helper functions, control blocks and lines.

    Recompute the oracle for each reduction and accept only wrong-value
    assertions. Rejected syntax, scope, or interpreter errors retain the line.
    This is a local minimum under deletion, not a globally shortest program.
    """
    changed = True
    while changed:
        changed = False
        candidates = []
        for m in re.finditer(r'fun h\d+\([^)]*\) -> u16 \{\n.*?\n\}(?:\n|$)', src, re.S):
            candidates.append(src[:m.start()] + src[m.end():])
        lines = src.splitlines(keepends=True)
        for i, line in enumerate(lines):
            if re.match(r'\s*(if|while) .* \{$', line):
                end = Interp(src).match([s.rstrip('\n') for s in lines], i)
                if lines[end].strip() == '} else {':
                    end = Interp(src).match([s.rstrip('\n') for s in lines], end)
                candidates.append(''.join(lines[:i] + lines[end+1:]))
            if re.match(r'\s*(let |var |\w+ = |return )', line):
                candidates.append(''.join(lines[:i] + lines[i+1:]))
        for candidate in candidates:
            try:
                program = with_assert(candidate, args, mode)
            except Exception:
                continue
            if mismatch(check(program)):
                src = candidate
                changed = True
                break
    return with_assert(src, args, mode)


def differential_status(mir, z80):
    if re.search(r'(?i)assembl|invalid instruction|unknown instruction', z80['error']): return 'assembly failure'
    # An unjudged backend must not be hidden by the other backend's wrong value.
    if any(not r['pass'] and not mismatch(r) for r in (mir, z80)): return 'compiler error'
    if mismatch(mir): return 'MIR2 != oracle'
    if mir['pass'] and mismatch(z80): return 'Z80 != MIR2'
    if mir['pass'] and z80['pass']: return 'pass'
    return 'compiler error'


def reduction_check(text, status, check):
    mir = check(text, 'mir2')
    if status == 'MIR2 != oracle':
        return mir
    if not mir['pass']:
        return {'pass': False, 'error': 'MIR2 no longer agrees with oracle'}
    return check(text)


def fuzz_one(seed, mz, timeout, output, reduce=True, mode="folded", signed=False, masked_loops=False):
    src, args = generated(seed, signed, masked_loops)
    try:
        program = with_assert(src, args, mode)
    except Exception as e:
        return {'seed': seed, 'status': 'oracle error', 'error': str(e)}
    with tempfile.TemporaryDirectory(prefix='fuzz-diff-') as temp:
        path = Path(temp) / 'case.nanz'
        def check(text, backend='z80'):
            path.write_text(text)
            return run_compiler(mz, path, timeout, backend=backend)
        mir = check(program, 'mir2')
        z80 = check(program)
        status = differential_status(mir, z80)
        result = mir if status == 'MIR2 != oracle' or (status == 'compiler error' and not mir['pass'] and not mismatch(mir)) else z80
        if status in ('MIR2 != oracle', 'Z80 != MIR2'):
            def reduce_check(text):
                return reduction_check(text, status, check)
            reduced = minimize(src, args, reduce_check, mode) if reduce else program
            (output / f'seed-{seed}.nanz').write_text(reduced)
            (output / f'seed-{seed}.original.nanz').write_text(program)
        elif status in ('compiler error', 'assembly failure'):
            (output / f'seed-{seed}.error.nanz').write_text(program)
        return {'seed': seed, 'status': status, 'error': result['error'], 'mir2': mir, 'z80': z80}


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--mz', required=True)
    p.add_argument('--seed', type=int, default=0)
    p.add_argument('--count', type=positive, default=500)
    p.add_argument('-j', type=positive, default=os.cpu_count() or 1)
    p.add_argument('--timeout', type=float, default=30)
    p.add_argument('--output', type=Path, required=True)
    p.add_argument('--no-reduce', action='store_true', help='save full reproducers and skip deletion reduction for fast triage')
    p.add_argument('--fail-on', choices=('all', 'backend'), default='all',
                   help='all: fail on any finding (default); backend: fail on backend/compiler failures or less than 95%% passing seeds; report all findings')
    p.add_argument('--mode', choices=('folded', 'direct-call'), default='folded',
                   help='folded: assert a constant-foldable wrapper; direct-call: execute f with assertion arguments')
    corpus = p.add_mutually_exclusive_group()
    corpus.add_argument('--masked-loops', action='store_true', help='opt in to masked byte predicates inside while loops; preserves default and signed seeds')
    corpus.add_argument('--signed', action='store_true', help='opt in to signed variables, negative literals and comparisons; preserves default seeds')
    a = p.parse_args()
    if a.timeout <= 0:
        p.error('--timeout must be positive')
    a.output.mkdir(parents=True, exist_ok=True)
    mz = str(Path(a.mz).resolve())
    with concurrent.futures.ThreadPoolExecutor(max_workers=a.j) as pool:
        results = list(pool.map(lambda seed: fuzz_one(seed, mz, a.timeout, a.output, not a.no_reduce, a.mode, a.signed, a.masked_loops), range(a.seed, a.seed+a.count)))
    (a.output / 'results.json').write_text(json.dumps(results, indent=2) + '\n')
    for status in ['pass', 'MIR2 != oracle', 'Z80 != MIR2', 'assembly failure', 'compiler error', 'oracle error']:
        print(f'{status}: {sum(r["status"] == status for r in results)}')
    for r in results:
        if r['status'] != 'pass':
            print(f'seed {r["seed"]}: {r["status"]}: {r["error"]}')
    passes = sum(r['status'] == 'pass' for r in results)
    floor_failed = passes * 100 < len(results) * 95
    print(f'Passing seeds: {passes}/{len(results)}; minimum 95%; mode: {a.mode}')
    blocking = {'Z80 != MIR2', 'assembly failure', 'compiler error'}
    return int(floor_failed or any(r['status'] != 'pass' if a.fail_on == 'all' else r['status'] in blocking
                   for r in results))


if __name__ == '__main__':
    try:
        raise SystemExit(main())
    except Exception as e:
        print(f'tool error: {e}', file=sys.stderr)
        raise SystemExit(2)
