#!/bin/bash

# review change
git diff
# Add all changes
git add .

# Commit changes with the provided commit message
commit_message="$1"
git commit -m "$commit_message"

# Push changes to the remote repository
git push
