I need to store information about this page for archiving purpose, creating a report of it.
I need a JSON answer from you. You can use markdown in values.
Read the opened current page to do a few separate things for me:
- summary: create a summary about it
- keypoints: a list of a few key points about the page, using format `**Keypoint title**: keypoint summary`
- tags: a list of a few tag about the article, use dash for multiple words (e.g. 'web', 'react', 'golang', 'architecture', 'web-app', the framework if any, essential keyword, do not be too specific, keep it simple), at most 5. The tags will help me archive and retrieve articles summary. Also a tag should be a word, of use a dash if multiple words, no caps all lower case.

To see what I expect of you, there is an example of a JSON answer, where value needs to be updated:

```json
{
    "summary": "This is the summary of the article",
    "keypoints": [
        "**keypoint 1**: summary of keypoint 1",
        "**keypoint 2**: summary of keypoint 2"
    ],
    "tags": [
        "tag1",
        "another-tag",
        "yet-another-tag"
    ]
}
```

The user will now provide you with the page content.