# Fork notice
This fork builds upon the work of [@nisanthchunduru](https://github.com/nisanthchunduru).

<img src="https://raw.githubusercontent.com/ma111e/hugo-notion/main/readme/preview.png" />
<img src="https://raw.githubusercontent.com/ma111e/hugo-notion/main/readme/preview_interactive.png" />

## Additions
+ TUI
+ Interactive mode to select which files to sync
+ CLI flags & config file
+ Remote image pull to host them locally
+ Compatible with a more structured post architecture
+ Better flexiblility overall

## Removal
+ Removed the repeat feature as it got in the way (check the original project if you need it—or use watch/cron)
+ Docker support (todo: restore it with a proper scheduling method)

# hugo-notion
`hugo-notion` converts Notion pages to markdown in a local directory.

## Installation
```
go install github.com/ma111e/hugo-notion@latest
```

## Setup

1. Create a Notion integration, generate a secret and connect that integration to the Notion page that will contain the pages to sync.
    > See https://developers.notion.com/docs/create-a-notion-integration#getting-started
1. Copy one of the sample configuration file in your Hugo directory and edit it to suit your needs 
   > `cp .env.sample .env`
   > 
   > `cp .hugo-notion.yml.sample .hugo-notion.yml`
1. Run hugo-notion
    > `hugo-notion`

### Configuration
You can either use a `.env` file, environment variables, or the `.hugo-notion.yml` file if you need a more advanced configuration management.

#### YAML defaults

```yaml
notion_root_page: https://www.notion.so/xxx-yyy-changeme
content_dir: ./content/posts
add_front_matter: false
interactive: false
notion_token: ntn_changeme
posts_base_uri: /posts
s3_images: false
```

#### ENV defaults

```dotenv
HN_NOTION_ROOT_PAGE=https://www.notion.so/xxx-yyy-changeme
HN_CONTENT_DIR=./content/posts
HN_ADD_FRONT_MATTER=false
HN_INTERACTIVE=false
HN_NOTION_TOKEN=ntn_changeme
HN_POSTS_BASE_URI=/posts
HN_S3_IMAGES=false
```

Every setting can be overridden with flags at runtime. See [Usage](#Usage) below.

## Usage
```yaml
Usage:
  hugo-notion [flags]

Flags:
  -a, --add-front-matter        add front matter in markdown files
  -c, --config string           config file (default is ./.hugo-notion.yml)
  -d, --content-dir string      content directory (default is ./content/posts) (default "./content/posts")
  -h, --help                    help for hugo-notion
  -i, --interactive             enable interactive page selection
      --posts-base-uri string   base URI for posts in the generated site (default "/")
      --s3-images               use S3 for image storage (legacy behavior)
  -t, --token string            Notion token of the integration connected to the root page to fetch
  -u, --url string              Notion page URL to sync```
```

## Bug reports

If you hit a bug, please do report it by creating a GitHub issue.

PR are welcome.

## Similar projects

The below are similar projects that didn't meet my needs or that I had discovered later

- https://github.com/dobassy/notion-hugo-exporter
