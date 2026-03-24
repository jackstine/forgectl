<spec-queue-format>
<description>JSON file that lists specifications to be written for a domain.</description>

<example>
```json
{
  "specs": [
    {
      "name": "Service Configuration",
      "domain": "launcher",
      "topic": "Configuration loading, validation, and default-value application for service endpoints.",
      "file": "launcher/specs/service-configuration.md",
      "planning_sources": ["launcher/.workspace/implementation_plan/notes/config.md"],
      "depends_on": []
    }
  ]
}
```
</example>

<fields>
  <field name="name" type="string">Human-readable name for the spec (e.g., "Service Configuration")</field>
  <field name="domain" type="string">The domain this spec belongs to (e.g., "launcher", "optimizer")</field>
  <field name="topic" type="string">One-sentence description of what the spec covers</field>
  <field name="file" type="string">Path where the spec file will be written, relative to project root (e.g., launcher/specs/service-configuration.md)</field>
  <field name="planning_sources" type="string[]">Paths to reference material the spec author should read. Can be empty.</field>
  <field name="depends_on" type="string[]">Name values of other specs in this file that must be written first. Can be empty.</field>
</fields>

<rules>
  - specs must be a non-empty array.
  - All 6 fields are required on every entry.
  - No additional fields are allowed.
  - Every value in depends_on must match a name in another entry.
</rules>
</spec-queue-format>
