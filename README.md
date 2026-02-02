# Ideasbglobot

An ultra-modular Telegram bot with heavy contextualization and learning capabilities.

## Features

- **Contextualization**: Uses a `CONTEXT.md` file and a SQLite database to maintain persistent context.
- **Learning**: The bot can learn new facts about users or the environment and store them in its database.
- **Self-Editing**: The bot can suggest updates to its own `CONTEXT.md` file to refine its identity or knowledge.
- **Multi-Platform**: Designed to support multiple chat platforms (currently focusing on Telegram).
- **Modular AI Providers**: Supports Ollama and VibeAura.

## Installation

You can install `ideasbglobot` using the following curl command:

```bash
curl -sSL https://raw.githubusercontent.com/nathfavour/ideasbglobot/main/install.sh | bash
```

## Updates

To update to the latest version, run:

```bash
ideasbglobot bot update
```

## Data Directory

The bot stores its data in `~/.ideasbglobot/`:
- `configs.json`: Bot tokens and general settings.
- `data.db`: SQLite database for messages and learned facts.
- `CONTEXT.md`: Persistent global context file.
- `process.json`: Task queue for long-running processes.

## Learning & Task Tags

The bot understands the following tags in its AI responses:
- `[LEARN: key=value]`: Stores a fact in the database for the current chat.
- `[UNLEARN: key]`: Removes a previously learned fact.
- `[UPDATE_CONTEXT: new content]`: Completely overwrites `CONTEXT.md` with new content.
- `[TASK: title | description | YYYY-MM-DD HH:MM]`: Schedules a reminder/task.

## Background Scheduler

The bot runs a background scheduler that checks for pending tasks every minute and sends reminders to the appropriate chat when a task is due (within a 5-minute window).

## Configuration

Run the bot to generate the default config. You can set the Telegram token in `~/.ideasbglobot/configs.json`.
You can also use the CLI to set the AI model:
```bash
go run main.go ai ollama model set
```
