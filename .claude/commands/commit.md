Please analyze the changes we have and prepare a commit. Always show a commit analysis and confirm the commit message.

**commit-message:** The proposed commit message.

## Instructions

1. **Analyze Changes**:
   - Run `git status` to see all untracked files.
   - Run `git diff` to see both staged and unstaged changes.
   - Run `git log` to see recent commit messages and follow the repository's commit message style.
   - Use the git context to determine relevant files. Add relevant untracked files to the staging area. Do not commit files that were already modified at the start of this conversation if they are not relevant to your commit.

2. **Draft Commit Message**:
   - Wrap your analysis process in `<commit_analysis>` tags.
   - List the changed or added files.
   - Summarize the nature of the changes (new feature, enhancement, bug fix, refactoring, test, docs, etc.).
   - Brainstorm the purpose or motivation.
   - Assess the impact.
   - Check for sensitive information.
   - Draft a concise (1-2 sentences) commit message focusing on the "why."
   - Ensure clarity, conciseness, and accuracy.
   - Ensure the message is not generic.

3. **Commit Changes**:
   - Use `git commit -m "$(cat <<'EOF' ... EOF)"` with `requires_confirmation: true`.
   - If the commit fails due to pre-commit hook changes, retry ONCE. If it fails again, it means a pre-commit hook is preventing the commit.
   - If the commit succeeds but files were modified by pre-commit hook, amend your commit to include them.

4. **Verify Commit**:
   - Run `git status` to ensure the commit succeeded.

## Context

- The command will operate on the current Git repository.
- It will analyze both staged and unstaged changes.
- It will follow the existing commit message conventions of the repository.

## Success Confirmation

After completing the task, please confirm:
- ✅ New command file created at `.claude/commands/commit.md`
- 📍 Command can be invoked as `/project:commit`
- 📝 Command follows proper Claude Code format with `commit-message` parameter
- 🔧 Command includes clear instructions and success confirmation
