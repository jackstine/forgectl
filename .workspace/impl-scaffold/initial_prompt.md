ok we are going to create a scaffold for implementation planning.  it
  will have a similar state machine

  ORIENT → SELECT → STUDY_SPECS -> STUDY_CODE -> STUDY_PACKAGES -> DRAFT
  → EVALUATE → (REFINE loop) → ACCEPT → DONE


  During the STUDY_SPECS the agent will be asked to study specs that are
  in the SPEC_MANIFEST.md
     - this could be the whole file or diffs associated with the specs,
  when they were saved in the code base.

  STUDY_CODE
  ```
     explore the code base using x number of sub agents pertaining the
  specs that we are looking at.
  ```
  place a TODO the number of sub agents is 3 for now.  But place a TODO
  in the specs, so that we know that we can change it later.

  STUDY_PACKAGES
  - study the packages of the codebase, it could be something like the
  context7 packages in the CLAUDE.md or something related to the
  package.yml or go.mod or package file for the codebase.

  then draft up the thing,  then have a sub agent run that will do the
  same thing as you did, and evaluate the implementation plan.


  Please create the specs for the application,  read the
  `.workspace/specs/SPEC_FORMAT.md` to understand the format of specs


  What does the Select phase do again?  i do not remember

  now we are going to write the specs for it here
  protocols/.workspace/implementation_plan
