# MinZ Documentation Guide

## 📚 Documentation System

All MinZ documentation follows a numbered system for easy tracking and organization.

### Directory Structure

```
minz-ts/
├── README.md           # Main project readme (never numbered)
├── TODO.md            # Current tasks (never numbered)
├── STATUS.md          # Project status (never numbered)
├── CLAUDE.md          # AI assistant guide (never numbered)
├── docs/              # All numbered documentation (001-999)
├── inbox/             # Drop new docs here for auto-numbering
└── organize_docs.sh   # Auto-numbering script
```

### Documentation Workflow

1. **Writing New Documentation**
   - Write your document with a descriptive name: `My_New_Feature_Guide.md`
   - Place it in the `inbox/` folder
   
2. **Auto-Numbering**
   - Run `./organize_docs.sh`
   - Script finds the next available number
   - Moves and renames: `inbox/My_New_Feature_Guide.md` → `docs/165_My_New_Feature_Guide.md`

3. **Batch Processing**
   - Place multiple documents in `inbox/`
   - Run script once to number them all sequentially

### Numbering Convention

- Format: `NNN_Document_Title.md`
- Numbers: 001-999 (three digits, zero-padded)
- Underscores replace spaces and special characters
- Sequential numbering preserves chronological order

### Special Documents (Never Numbered)

These remain in the root directory:
- `README.md` - Project introduction
- `TODO.md` - Active task tracking
- `STATUS.md` - Current project status
- `CLAUDE.md` - AI assistant instructions
- `CHANGELOG.md` - Version history
- `CONTRIBUTING.md` - Contribution guidelines
- `LICENSE` - Legal information

### Finding Documents

```bash
# List all docs by number
ls docs/ | sort -n

# Find specific topic
grep -l "TSMC" docs/*.md

# Show recent additions
ls -lt docs/ | head -10

# Find by number range
ls docs/15[0-9]_*.md  # Docs 150-159
```

### Document Categories

By number ranges (convention):
- 001-099: Early development, architecture
- 100-149: Feature implementations
- 150-199: Analysis and audits
- 200-299: Release notes and changelogs
- 300-399: Tutorials and guides
- 400-499: Research and experiments
- 500+: Future expansions

### Examples

```bash
# Add a new design document
echo "# New Design" > inbox/Iterator_Design_v2.md
./organize_docs.sh
# Creates: docs/165_Iterator_Design_v2.md

# Add multiple docs
cp *.md inbox/
./organize_docs.sh
# Numbers them all sequentially
```

### Benefits

1. **Chronological Ordering** - Document history preserved
2. **No Naming Conflicts** - Numbers are unique
3. **Easy References** - "See doc 147 for details"
4. **Simple Automation** - Scripts can process by number
5. **Clear History** - Evolution of project visible

### Current Statistics

- Total Documents: 164
- Next Available: 165
- Latest Additions:
  - 155: Action Plan from Audit
  - 156: Actual State Report
  - 157: Architecture Audit

---

*Use `./organize_docs.sh` to maintain documentation consistency!*