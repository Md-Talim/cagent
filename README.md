[![progress-banner](https://backend.codecrafters.io/progress/claude-code/807b8e8f-5b9f-4d82-b661-e9df66078c7a)](https://app.codecrafters.io/users/codecrafters-bot?r=2qF)

# Building Claude Code from Scratch

I'm learning agentic engineering by building a small Claude Code clone through the [CodeCrafters "Build Your Own Claude Code" challenge](https://codecrafters.io/challenges/claude-code).

The basic idea: send a prompt to an LLM, let the LLM call tools like reading a file or running a shell command, run those tools, send the results back, and keep looping until the model gives a final answer.

## What it can do right now

- Read a file
- Write a file
- Run a bash command
- Loop through tool calls until the model stops asking for them

The tool names the model sees are `Read`, `Write`, and `RunBashCommand`.

## Running it

To be honest, I mostly run this through the CodeCrafters tests and have not spent much time running it locally. If you want to try it locally:

1. Install Go 1.25.
2. Get an API key from [OpenRouter](https://openrouter.ai).
3. Set `OPENROUTER_API_KEY`. You can also set `OPENROUTER_BASE_URL` if you want to point at a different OpenAI compatible endpoint. It defaults to `https://openrouter.ai/api/v1`.
4. Run:

```sh
./run.sh -p "your prompt here"
```

I have not added a `.env` loader, so the key needs to be in your environment. Don't commit your key.

Be a little careful with this: the agent can write files and run bash commands. Try it in a scratch directory and don't put secrets in your prompts.

## Tests

```sh
go test ./...
go vet ./...
```
