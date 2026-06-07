<role></role>
<task></task>
<workflow>
  <step_0></step_0>
  <step_1 cond="..."></step_1>
  <loop_2 over="each_x in xs">
    <step_2_0></step_2_0>
    <if_2_1 cond="...">
      <step_2_1_0></step_2_1_0>
    <else>
      <step_2_1_0></step_2_1_0>
    </else>
    </if_2_1>
    <loop_2_2 until="...">
      <step_2_2_0></step_2_2_0>
      <step_2_2_N></step_2_2_N>
    </loop_2_2>
  </loop_2>
  <step_N></step_N>
</workflow>
<contextual_information>provides context about the workflow in markdown</contextual_information>
<IMPORTANT_INFO>where information is denoted by 999 with each importance going up by one digit added</IMPORTANT_INFO>

<IMPORTANT_INFO>
999. have fun
9999. you have got this!
99999. do not stress out
</IMPORTANT_INFO>

name = <type>_<path>. type is step | loop | if. path is the positional coordinate from the root, one index per level. steps and loops share one zero-based counter per container.

<step_3 cond="cache_exists"></step_3>  // guard: runs only if cond is true

<loop_2 over="each_x in xs">
  <step_2_0></step_2_0>
  <loop_2_2 until="done">
    <step_2_2_0></step_2_2_0>
  </loop_2_2>
</loop_2>

<if_5 cond="lockfile_changed">
  <step_5_0></step_5_0>
<else>
  <step_5_0></step_5_0>
</else>
</if_5>

if you need to create a reference material to explain anything, add this to references folder and provide a file reference to the document.
reference files need to have the same structure format.
