#!/bin/bash

differences_found=0

# Extract keys from the reference file and sort them
sort <(awk -F':' '{print $1}' internal/i18n/locales/en-US.yml) > sorted_reference_keys.txt
sed -i '/^\s*$/d' sorted_reference_keys.txt

for new_file in internal/i18n/locales/*.yml; do
    filename=$(basename $new_file)
    echo ""
    echo "Checking $filename..."

    # Extract keys from the current file and sort them
    sort <(awk -F':' '{print $1}' $new_file) > sorted_new_keys.txt
    sed -i '/^\s*$/d' sorted_new_keys.txt

    comm -3 sorted_reference_keys.txt sorted_new_keys.txt > differences.txt

    if [ -s differences.txt ]; then
        echo "Error in $filename: The YAML file has differences in keys."
        while IFS= read -r line; do
            if [[ $line == $'\t'* ]]; then
                echo "+ Additional key in $filename: $(echo $line | awk '{$1=$1; print}')"
            else
                echo "- Missing key in $filename: $(echo $line | awk '{$1=$1; print}')"
            fi
        done < differences.txt
        differences_found=1
    else
        echo "All keys in $filename match perfectly."
    fi

    rm sorted_new_keys.txt
done

rm sorted_reference_keys.txt differences.txt

exit $differences_found
