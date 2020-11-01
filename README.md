[![CircleCI](https://circleci.com/gh/giantswarm/template-operator.svg?&style=shield)](https://circleci.com/gh/giantswarm/template-operator)
[![Docker Repository on Quay](https://quay.io/repository/giantswarm/template-operator/status "Docker Repository on Quay")](https://quay.io/repository/giantswarm/template-operator)

# REPOSITORY_NAME

This is a template repository containing files for a giantswarm
operator repository.

To use it just hit `Use this template` button or [this
link][generate].

1. Run`devctl replace -i "REPOSITORY_NAME" "$(basename $(git rev-parse
   --show-toplevel))" --ignore '.git/**' '**'`.
2. Run `devctl replace -i "template-operator" "$(basename $(git rev-parse
   --show-toplevel))" --ignore '.git/**' '**'`.
3. Go to https://github.com/giantswarm/REPOSITORY_NAME/settings and make sure `Allow
   merge commits` box is unchecked and `Automatically delete head branches` box
   is checked.
4. Go to https://github.com/giantswarm/REPOSITORY_NAME/settings/access and add
   `giantswarm/bots` with `Write` access and `giantswarm/employees` with
   `Admin` access.
5. Add this repository to https://github.com/giantswarm/github.
6. Create quay.io docker repository if needed.
7. Add the project to the CircleCI:
   https://circleci.com/setup-project/gh/giantswarm/REPOSITORY_NAME
8. Change the badge (with style=shield):
   https://circleci.com/gh/giantswarm/REPOSITORY_NAME.svg?style=shield&circle-token=TOKEN_FOR_PRIVATE_REPO
   If this is a private repository token with scope `status` will be needed.

[generate]: https://github.com/giantswarm/template-operator/generate
