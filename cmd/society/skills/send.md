---
name: society:send
description: Send a message to a society agent and display the response
---

# society:send

Send a message to a registered society agent and display the response.

## Usage

```
/society:send <agent> <message>
```

## Arguments

- `agent` — Name of the registered agent (e.g., "claude", "echo", "arch-claude")
- `message` — The message to send

## Instructions

When the user invokes `/society:send <agent> <message>`:

### 1. Verify the agent exists

```bash
society list
```

If the agent is not in the list, suggest running `society onboard --auto` or `society onboard`.

### 2. Send the message

```bash
society send <agent> "<message>"
```

### 3. Display the response

Show the agent's response to the user. If the response includes code or structured output, format it appropriately.

### 4. Thread continuity

If the user wants to continue the conversation, note the thread ID from the response and use it:

```bash
society send --thread <thread-id> <agent> "<follow-up message>"
```
