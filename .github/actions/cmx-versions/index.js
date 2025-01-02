const core = require('@actions/core');
const fetch = require('node-fetch');
const semverCoerce = require('semver/functions/coerce');
const semverGt = require('semver/functions/gt');

async function getClusterVersions() {
    const url = 'https://api.replicated.com/vendor/v3/cluster/versions';
    const apiToken = core.getInput('replicated-api-token') || process.env.REPLICATED_API_TOKEN;
    const headers = {
        Authorization: apiToken
    };

    let clusterVersions = [];
    try {
        const response = await fetch(url, {
            method: 'GET',
            headers,
        });

        if (response.status === 200) {
            const payload = await response.json();
            clusterVersions = payload['cluster-versions'];
        } else {
            throw new Error(`Request failed with status code ${response.status}`);
        }
    } catch (error) {
        console.error(`Error: ${error.message}`);
        core.setFailed(error.message);
        return;
    }

    let distros = JSON.parse(core.getInput('include-distributions'));

    // versions to test looks like this:
    // [
    //   {distribution: k3s, version: v1.24},
    //   {distribution: eks, version: v1.28},
    //   ...
    // ]
    const versionsToTest = [];

    clusterVersions.forEach((distribution) => {
        const distroName = distribution.short_name;
        if (!distros.includes(distroName)) {
            // skip this distribution
            return;
        }

        latestVersion = getLatestVersion(distribution);

        versionsToTest.push({ distribution: distroName, version: latestVersion });
    });

    console.log(versionsToTest);
    core.setOutput('versions-to-test', JSON.stringify(versionsToTest));
}

function getLatestVersion(distribution) {
    let latestVersion = undefined;
    distribution.versions.forEach((version) => {
        if (latestVersion === undefined) {
            latestVersion = version;
        } else {
            const parsed = semverCoerce(version);
            if (semverGt(parsed, semverCoerce(latestVersion))) {
                latestVersion = version;
            }
        }
    });

    return latestVersion;
}

getClusterVersions();
