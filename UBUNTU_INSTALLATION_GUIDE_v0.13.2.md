# MinZ v0.13.2 Ubuntu Installation Guide

## 🎯 Zero-Dependency Installation for Ubuntu/Debian

MinZ v0.13.2 **completely eliminates** the Ubuntu installation issues with embedded parsers and zero external dependencies.

## 🚀 Quick Installation (Recommended)

### Option 1: Native Parser Build (Fastest Performance)

```bash
# Download and install in one command
curl -L https://github.com/oisee/minz/releases/download/v0.13.2/minz-v0.13.2-linux-amd64.tar.gz | tar -xz && cd minz-v0.13.2-linux-amd64 && sudo cp mz /usr/local/bin/ && mz --version
```

### Option 2: ANTLR-Only Build (Maximum Compatibility)

```bash
# For Docker, CI/CD, or CGO-free environments
curl -L https://github.com/oisee/minz/releases/download/v0.13.2/minz-v0.13.2-linux-amd64-antlr-only.tar.gz | tar -xz && cd minz-v0.13.2-linux-amd64-antlr-only && sudo cp mz /usr/local/bin/ && mz --version
```

## 📋 Step-by-Step Installation

### Step 1: Download MinZ v0.13.2

Choose the appropriate build for your needs:

#### For Maximum Performance (Includes both Native + ANTLR parsers)
```bash
wget https://github.com/oisee/minz/releases/download/v0.13.2/minz-v0.13.2-linux-amd64.tar.gz
```

#### For Maximum Compatibility (ANTLR parser only, CGO-free)
```bash
wget https://github.com/oisee/minz/releases/download/v0.13.2/minz-v0.13.2-linux-amd64-antlr-only.tar.gz
```

#### For ARM64 Systems
```bash
# With both parsers
wget https://github.com/oisee/minz/releases/download/v0.13.2/minz-v0.13.2-linux-arm64.tar.gz

# ANTLR-only
wget https://github.com/oisee/minz/releases/download/v0.13.2/minz-v0.13.2-linux-arm64-antlr-only.tar.gz
```

### Step 2: Extract the Archive

```bash
# Extract (replace with your downloaded file)
tar -xzf minz-v0.13.2-linux-amd64.tar.gz
cd minz-v0.13.2-linux-amd64
```

### Step 3: Install MinZ

#### System-wide Installation (Recommended)
```bash
sudo cp mz /usr/local/bin/
```

#### User-only Installation
```bash
mkdir -p ~/.local/bin
cp mz ~/.local/bin/
# Add to PATH if not already added
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

### Step 4: Verify Installation

```bash
mz --version
```

You should see output like:
```
MinZ v0.13.2
Parser: Native + ANTLR (dual parser support)
Platform: linux/amd64
```

## 🎯 Parser Selection Guide

### Default Behavior (Automatic)
```bash
# Uses fastest available parser automatically
mz program.minz -o program.a80
```

### Force Native Parser (Maximum Performance)
```bash
# Explicitly use tree-sitter native parser
MINZ_USE_NATIVE_PARSER=1 mz program.minz -o program.a80
```

### Force ANTLR Parser (Maximum Compatibility)
```bash
# Explicitly use pure-Go ANTLR parser
MINZ_USE_ANTLR_PARSER=1 mz program.minz -o program.a80
```

### Check Which Parser is Active
```bash
# The compiler will show which parser it's using
mz --parser-info
```

## 🧪 Quick Test

Create and compile your first MinZ program:

```bash
# Create a simple program
cat > hello.minz << 'EOF'
fun main() -> u8 {
    print("Hello from MinZ v0.13.2!");
    return 42;
}
EOF

# Compile it
mz hello.minz -o hello.a80

# Success! You should see output like:
# Compiled hello.minz -> hello.a80 (using Native parser, 1.2ms)
```

## 🆘 Troubleshooting

### "Command not found: mz"

**Solution**: Add to PATH
```bash
# Check if installed correctly
ls -la /usr/local/bin/mz
# OR
ls -la ~/.local/bin/mz

# Add to PATH if using local install
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

### "Permission denied" on /usr/local/bin

**Solution**: Use local installation
```bash
mkdir -p ~/.local/bin
cp mz ~/.local/bin/
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

### Parser Issues

**Problem**: Native parser fails
```bash
# Automatically falls back to ANTLR parser
mz program.minz -o program.a80

# Or force ANTLR parser
MINZ_USE_ANTLR_PARSER=1 mz program.minz -o program.a80
```

**Problem**: "CGO error" or "C compiler not found"
```bash
# Use ANTLR-only build (no CGO required)
wget https://github.com/oisee/minz/releases/download/v0.13.2/minz-v0.13.2-linux-amd64-antlr-only.tar.gz
```

### Docker/Container Issues

Use the ANTLR-only build for containers:
```dockerfile
FROM ubuntu:22.04
RUN apt-get update && apt-get install -y curl
RUN curl -L https://github.com/oisee/minz/releases/download/v0.13.2/minz-v0.13.2-linux-amd64-antlr-only.tar.gz | tar -xz -C /usr/local/bin --strip-components=1
```

## 🚧 Platform-Specific Notes

### Ubuntu 20.04+ / Debian 11+
- Both builds work perfectly
- Native parser recommended for best performance

### Ubuntu 18.04 / Debian 10
- ANTLR-only build recommended
- Native parser may require newer glibc

### WSL (Windows Subsystem for Linux)
- Both builds work
- ANTLR-only recommended for better compatibility

### Docker Containers
- Always use ANTLR-only builds
- No external dependencies required

### CI/CD Environments
- Use ANTLR-only builds for reliable builds
- No CGO compilation required

## 🎯 Performance Recommendations

### For Development (Local Machine)
- **Use**: Native parser build (`minz-v0.13.2-linux-amd64.tar.gz`)
- **Why**: 15-50x faster parsing, better development experience

### For Production Deployment
- **Use**: ANTLR-only build (`minz-v0.13.2-linux-amd64-antlr-only.tar.gz`)
- **Why**: No CGO dependencies, more reliable in containers

### For CI/CD Pipelines
- **Use**: ANTLR-only build
- **Why**: Simpler build requirements, faster container images

## 🆘 Getting Help

### Report Issues
When reporting issues, please include:
```bash
# System information
uname -a
mz --version
mz --parser-info

# Error details
mz your_program.minz -v -o output.a80
```

### Community Support
- **GitHub Issues**: https://github.com/oisee/minz/issues
- **Discussions**: https://github.com/oisee/minz/discussions

## 🎉 Success!

You now have MinZ v0.13.2 installed with:
- ✅ Zero external dependencies
- ✅ Dual parser support (Native + ANTLR)
- ✅ Complete Ubuntu compatibility
- ✅ Ready for development

**Next Steps**: Check out the [MinZ Language Guide](README.md) to start programming!

---

*MinZ v0.13.2 - The Ubuntu installation problem is finally solved!* 🚀