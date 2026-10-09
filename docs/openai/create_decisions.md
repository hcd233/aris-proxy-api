## Create a decision

**post** `/decisions`

Use this endpoint to ask classification or scoring questions about the same input. You’ll get the answers back in the order you asked the questions.

For text, you can pass a string. You can also send user messages containing `input_text` and `input_image` parts, with up to 128 images per request. Images can be base64 data URLs or publicly accessible HTTP(S) URLs. File IDs aren’t accepted. Other message roles, function calls, files, audio, and item references aren’t supported.

Sometimes a question returns a refusal instead of an answer. The result has type `refusal` and includes the question’s name, or `null` if you didn’t give it one.

### Body Parameters

- `input: string or array of DecisionInputMessage`

  The text or images to evaluate for every question. Provide a text string or user messages containing text and images. Images can be base64 data URLs or publicly accessible HTTP(S) URLs; at most 128 images are allowed across all messages in one request. Files, audio, tools, and item references are not supported.

  - `string`

  - `array of DecisionInputMessage`

    - `content: string or array of DecisionInputPart`

      Text evidence or an ordered list of text and image parts.

      - `string`

      - `Parts = array of DecisionInputPart`

        - `DecisionInputText object { text, type }`

          - `text: string`

          - `type: "input_text"`

            - `"input_text"`

        - `DecisionInputImage object { image_url, type, detail }`

          An image provided as a base64 data URL or a publicly accessible HTTP(S) URL. File IDs are not supported.

          - `image_url: string`

            A base64-encoded image in a data URL or a publicly accessible HTTP(S) image URL.

          - `type: "input_image"`

            - `"input_image"`

          - `detail: optional "low" or "high" or "auto" or "original" or null`

            The image detail level, using the selected model's image profile. Defaults to auto.

            - `"low"`

            - `"high"`

            - `"auto"`

            - `"original"`

    - `role: "user"`

      - `"user"`

    - `type: optional "message"`

      - `"message"`

- `model: string`

- `questions: array of Predicate { instructions, type, name }  or Choice { choices, instructions, type, name }  or Score { instructions, levels, type, name }`

  - `Predicate object { instructions, type, name }`

    Estimate how likely it is that a statement about the input is true.

    - `instructions: string`

    - `type: "predicate"`

      The type of the object. Always `predicate`.

      - `"predicate"`

    - `name: optional string`

  - `Choice object { choices, instructions, type, name }`

    Choose from the supplied options based on the input.

    - `choices: array of object { value, description }`

      Provide between 2 and 255 choices. Each choice must be unique.

      - `value: string or boolean`

        Choice values are typed: a string and a boolean with the same text are distinct.

        - `string`

        - `boolean`

      - `description: optional string`

    - `instructions: string`

    - `type: "choice"`

      The type of the object. Always `choice`.

      - `"choice"`

    - `name: optional string`

  - `Score object { instructions, levels, type, name }`

    Rate the input against the supplied ordered levels.

    - `instructions: string`

    - `levels: array of object { label, description }`

      - `label: string`

      - `description: optional string`

    - `type: "score"`

      The type of the object. Always `score`.

      - `"score"`

    - `name: optional string`

- `safety_identifier: optional string or null`

  Opaque caller-provided end-user identifier, scoped by the verified org. Match Responses' limit; this is never the authenticated user identity.

### Returns

- `Decision object { answers, model, usage }`

  - `answers: array of Predicate { name, probability, type }  or Choice { choice, confidence, name, 2 more }  or Score { confidence, name, probabilities, 2 more }  or Refusal { name, type }`

    - `Predicate object { name, probability, type }`

      - `name: string or null`

      - `probability: number`

      - `type: "predicate"`

        The type of the object. Always `predicate`.

        - `"predicate"`

    - `Choice object { choice, confidence, name, 2 more }`

      - `choice: string or boolean`

        Choice values are typed: a string and a boolean with the same text are distinct.

        - `string`

        - `boolean`

      - `confidence: number`

      - `name: string or null`

      - `probabilities: array of object { probability, value }`

        - `probability: number`

        - `value: string or boolean`

          Choice values are typed: a string and a boolean with the same text are distinct.

          - `string`

          - `boolean`

      - `type: "choice"`

        The type of the object. Always `choice`.

        - `"choice"`

    - `Score object { confidence, name, probabilities, 2 more }`

      - `confidence: number`

      - `name: string or null`

      - `probabilities: array of object { label, probability, value }`

        - `label: string`

        - `probability: number`

        - `value: number`

      - `score: number`

      - `type: "score"`

        The type of the object. Always `score`.

        - `"score"`

    - `Refusal object { name, type }`

      The model declined to answer this question. Other questions in the same request can still receive answers.

      - `name: string or null`

      - `type: "refusal"`

        The type of the object. Always `refusal`.

        - `"refusal"`

  - `model: string`

  - `usage: object { input_tokens, input_tokens_details, output_tokens, 2 more }`

    - `input_tokens: number`

    - `input_tokens_details: object { cache_write_tokens, cached_tokens }`

      - `cache_write_tokens: number`

      - `cached_tokens: number`

    - `output_tokens: number`

    - `output_tokens_details: object { reasoning_tokens }`

      - `reasoning_tokens: number`

    - `total_tokens: number`

### Example

```http
curl https://api.openai.com/v1/decisions \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer $OPENAI_API_KEY" \
    -d '{
          "input": "string",
          "model": "model",
          "questions": [
            {
              "instructions": "instructions",
              "type": "predicate"
            }
          ]
        }'
```

#### Response

```json
{
  "answers": [
    {
      "name": "name",
      "probability": 0,
      "type": "predicate"
    }
  ],
  "model": "model",
  "usage": {
    "input_tokens": -2147483648,
    "input_tokens_details": {
      "cache_write_tokens": -2147483648,
      "cached_tokens": -2147483648
    },
    "output_tokens": -2147483648,
    "output_tokens_details": {
      "reasoning_tokens": -2147483648
    },
    "total_tokens": -2147483648
  }
}
```

### Example

```http
curl https://api.openai.com/v1/decisions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -d '{
    "model": "gpt-6-luna",
    "input": "The package arrived with a broken screen.",
    "questions": [
      {
        "type": "predicate",
        "name": "damaged",
        "instructions": "Does the customer report a damaged item?"
      }
    ]
  }'
```

#### Response

```json
{
  "model": "gpt-6-luna",
  "answers": [
    {"type": "predicate", "name": "damaged", "probability": 0.95}
  ],
  "usage": {
    "input_tokens": 42,
    "input_tokens_details": {"cached_tokens": 0, "cache_write_tokens": 0},
    "output_tokens": 0,
    "output_tokens_details": {"reasoning_tokens": 0},
    "total_tokens": 42
  }
}
```
