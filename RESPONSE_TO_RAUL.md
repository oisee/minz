# Response to Raúl

Hi Raúl! 👋

Thank you so much for reporting the installation issue on Ubuntu! Your feedback helped us identify and fix a critical problem that was affecting all Linux users.

## 🎉 Fixed in v0.13.1!

We've just released **MinZ v0.13.1** which specifically addresses your issue:
https://github.com/oisee/minz/releases/tag/v0.13.1

## 📦 What's Fixed

1. **No more "Expected source code but got an atom" errors!**
2. **Automatic dependency installation** - The release now includes a script that installs everything you need
3. **Clear error messages** - If something's missing, MinZ tells you exactly what and how to fix it

## 🚀 How to Install on Ubuntu

```bash
# 1. Download the Linux version
wget https://github.com/oisee/minz/releases/download/v0.13.1/minz-v0.13.1-linux-amd64.tar.gz

# 2. Extract it
tar -xzf minz-v0.13.1-linux-amd64.tar.gz
cd linux-amd64

# 3. Install dependencies (one-time setup)
./install-dependencies.sh
# This will install tree-sitter CLI which MinZ needs for parsing

# 4. Install MinZ
./install.sh

# 5. Test it!
mz examples/fibonacci.minz -o fibonacci.a80
```

## 🎯 Your hello.minz Should Work Now!

After installation, you can compile your hello.minz:
```bash
mz hello.minz -o hello.a80
```

## 🙏 Thank You!

Your bug report was incredibly helpful. You helped us:
- Identify a missing dependency issue
- Improve our error messages
- Make MinZ more accessible for Linux users
- Create better installation documentation

If you encounter any other issues or have suggestions, please don't hesitate to report them!

## 📚 What's New in v0.13.0/v0.13.1

Besides the fix, MinZ v0.13.0 introduced:
- **Module System** - Import and organize code with `import std;` and `import math as m;`
- **85% compilation success rate** - Most examples now work!
- **Standard Library** - Math functions and more
- **Zero-cost abstractions** - Modern features with vintage performance

Happy coding with MinZ! 🚀

Best regards,
The MinZ Team