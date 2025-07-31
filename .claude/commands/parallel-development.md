---
description: Execute multiple development tasks in parallel using AI agents
allowed_tools: ["Task", "TodoWrite"]
---

# Parallel Development Command

This command orchestrates multiple AI agents to work on different tasks simultaneously, achieving rapid development velocity.

## Execution Pattern

1. **Task Analysis**
   - Break down the request into independent components
   - Identify which tasks can be parallelized
   - Create clear specifications for each task

2. **Agent Deployment**
   ```
   For each independent task:
   - Deploy specialized agent with Task tool
   - Provide complete context and requirements
   - Let agent work autonomously
   ```

3. **Coordination**
   While agents work:
   - Update TodoWrite tracking
   - Review completed work
   - Commit and push changes
   - Deploy next agents

## Best Practices:
- **Clear task boundaries** - Each agent needs well-defined scope
- **Minimal dependencies** - Tasks should be as independent as possible
- **Trust the agents** - Don't micromanage, review results
- **Iterative deployment** - Start next tasks as others complete

## Example Usage:
"I need to implement feature X, fix bug Y, and add tests for Z"

This becomes:
- Agent 1: Implement feature X
- Agent 2: Fix bug Y  
- Agent 3: Add tests for Z

All three work simultaneously!

$ARGUMENTS