$:.push File.expand_path("lib", __dir__)

require "hugo_notion/version"

# Describe your gem and declare its dependencies:
Gem::Specification.new do |s|
  s.name          = "hugo-notion"
  s.version       = HugoNotion::VERSION
  s.authors       = ["Nisanth Chunduru"]
  s.email         = ["nisanth074@gmail.com"]
  s.homepage      = "https://github.com/nisanthchunduru/hugo-notion"
  s.summary       = "Write in Notion. Publish with Hugo."
  s.description   = "Write in Notion. Publish with Hugo."

  s.files = Dir["{lib}/**/*", "README.md"]

  s.add_dependency "httparty"
  s.add_dependency "notion_to_md"
  s.add_dependency "yaml"
  s.add_development_dependency "pry"
end
