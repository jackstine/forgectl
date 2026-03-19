 study @".workspace/specs/scaffold/specs/scaffold-cli.md"

  We are going to create a implementation scaffold.   this will be used for implementation.

    will have a similar state machine

    ORIENT → READ_STEPS -> CONFIRM -> (back to READ_STEPS) → (after 5 iterations) EVALUATE → ACCEPT → DONE

  The CLI will not have the next command, instead when calling advance, it will show the next step on the command

  We will call the user the Senior Software Engineer

    During the READ_STEPS
  we will output a number of steps for the software engineer to implement (the number is decided on init of the
  program)
  when they come back they have to confirm that the code is implemented and passes.  (CONFIRM)

  CONFIRM must ask the user of the cli to confirm,  so that might be two steps.

  after confirm it will show the list of next steps (it will repeat this a number of times selected on init)

  then it will go to evaluate state.
    - during this state the agent will ask the agent evaluate the code  that was created against the list of steps
  presented to them.

  then the agent need to confirm that this has been completed with the evaluate state.


  the program will run until it has completed the entire JSON list of items.  then it will go to a EVALUATE step,
  and finish up the remaining last items.
  then it will be in DONE.

