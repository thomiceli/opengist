# Import Gists from GitHub

After running Opengist at least once, you can import your Gists from GitHub using this script:

```shell
github_user=user # replace with your GitHub username
opengist_url="http://user:password@opengist.url/init" # replace user, password and Opengist url

curl -s https://api.github.com/users/"$github_user"/gists?per_page=100 | jq '.[] | .git_pull_url' -r | while read url; do 
    git clone "$url"
    repo_dir=$(basename "$url" .git)
    
    # Add remote, push, and remove the directory
    if [ -d "$repo_dir" ]; then
        cd "$repo_dir"
        git remote add gist "$opengist_url"
        git push -u gist --all
        cd ..
        rm -rf "$repo_dir"
    fi
done
```

