# Forgectl
Forgectl is a spec driven development harness. Takes plan to implementation completeness.


## Workflow
Take a plan and pipe it into your LLM coding agent.
The agent will then 
1. crete/update specs according to the changes in the plan
2. create a implementation plan
3. create implementation from the plan


## Pipelines
- Specify - take a plan and align specs to the plan
- Implemenation Planning - Plan how to implement the changes in specs to the existing code base.
- Implement - take an implementation plan, and implement it.


## Future Work
- Reverse Engineer implemenation into Specs for brownfield development repos
- Front End Implementation Agents
  - using playwright agents
- Currently Eval agents are general_purpose, so create  multiple eval agents for their specific tasks


## Skills
### Planner

Use the Planner Skill to plan, this allows you to currently create plans that will shape your specs.
It will help you tailor plans that have less defects and more accurate and solid logic.
- used to table ideas in the `tabled` section of the workspace


## Thanks and References
- Clayton Farr and Geoffery Huntley for implementation of a spec driven development workflow https://github.com/ClaytonFarr/ralph-playbook for the ralph playbook, and helping me harness AI potential.  https://github.com/ghuntley/how-to-ralph-wiggum .
- Jack Clark at Anthropic for helping me understand Anthropics work on scaffolding in late December that started this search for AI improvements. https://jack-clark.net/ .



