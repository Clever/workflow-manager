package resources

const awsExampleChoiceStateMachine = `{
    "Comment": "An example of the Amazon States Language using a choice state.",
    "StartAt": "FirstState",
    "States": {
        "FirstState": {
            "Type": "Task",
            "Resource": "arn:aws:lambda:REGION:ACCOUNT_ID:function:FUNCTION_NAME",
            "Next": "ChoiceState"
        },
        "ChoiceState": {
            "Type": "Choice",
            "Choices": [
                {
                    "Variable": "$.foo",
                    "NumericEquals": 1,
                    "Next": "FirstMatchState"
                },
                {
                    "Variable": "$.foo",
                    "NumericEquals": 2,
                    "Next": "SecondMatchState"
                }
            ],
            "Default": "DefaultState"
        },
        "FirstMatchState": {
            "Type": "Task",
            "Resource": "arn:aws:lambda:REGION:ACCOUNT_ID:function:OnFirstMatch",
            "Next": "NextState"
        },
        "SecondMatchState": {
            "Type": "Task",
            "Resource": "arn:aws:lambda:REGION:ACCOUNT_ID:function:OnSecondMatch",
            "Next": "NextState"
        },
        "DefaultState": {
            "Type": "Fail",
            "Error": "DefaultStateError",
            "Cause": "No Matches!"
        },
        "NextState": {
            "Type": "Task",
            "Resource": "arn:aws:lambda:REGION:ACCOUNT_ID:function:FUNCTION_NAME",
            "End": true
        }
    }
}`

const awsExampleChoiceStateMachineInvalid = `{
  "StartAt": "ChoiceState",
  "States": {
    "ChoiceState": {
      "Type" : "Choice",
      "Default": "DoesntExist",
      "Choices": [
        {
          "Variable": "$.foo",
          "NumericEquals": 1,
          "Next": "FirstMatchState"
        },
        {
          "Variable": "$.foo",
          "NumericEquals": 2,
          "Next": "SecondMatchState"
        }
      ]
    },
   "FirstMatchState": {
      "Type" : "Task",
      "Resource": "arn:aws:lambda:REGION:ACCOUNT_ID:function:OnFirstMatch",
      "End": true
    },
   "SecondMatchState": {
      "Type" : "Task",
      "Resource": "arn:aws:lambda:REGION:ACCOUNT_ID:function:OnSecondMatch",
      "End": true
    }
  }
}`
