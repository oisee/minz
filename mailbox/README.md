# 📬 MinZ Team Mailbox System

## How It Works

### Sending Messages
Messages are named: `YYYY-MM-DD-HHMM-to-<recipient>.md`

Example: `2025-08-16-1430-to-mza-colleague.md`

### Recipients
- `mza-colleague` - The MZA improvement specialist
- `compiler-team` - MinZ compiler team (main codebase)
- `alice` - Project lead (you!)
- `claude` - Me, working on various aspects

### Message Format
```markdown
# Subject Line Here
**From:** sender-name  
**To:** recipient-name  
**Date:** 2025-08-16 14:30  
**Priority:** HIGH/MEDIUM/LOW  

## Message Content

Your message here...

## Expected Response

How/when you expect a response:
- [ ] Acknowledge receipt
- [ ] Need input on specific question
- [ ] FYI only, no response needed

## Response Method
- Reply via: `YYYY-MM-DD-HHMM-to-sender.md`
- Or update status in: `status/<topic>.md`
```

### Checking Messages
- Check regularly when starting work
- Look for messages addressed to you
- Archive old messages to `mailbox/archive/` after reading

### Status Updates
For ongoing work, update files in `mailbox/status/`:
- `phase2-status.md` - MZA Phase 2 progress
- `compiler-fixes.md` - MinZ compiler bug fixes
- `feature-progress.md` - New feature implementation

## Current Team Members

1. **mza-colleague** - Working on MZA assembler improvements
2. **compiler-team** - MinZ compiler development  
3. **claude** - Multi-role assistant
4. **alice** - Project lead

## Mailbox Etiquette

1. Keep messages concise and actionable
2. Use clear subject lines
3. Specify expected response clearly
4. Archive messages older than 1 week
5. Use status files for ongoing work updates

---
*Mailbox system established: 2025-08-16*