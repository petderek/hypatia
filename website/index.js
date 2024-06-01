import {h, render} from 'https://esm.sh/preact';
import htm from 'https://esm.sh/htm';

const html = htm.bind(h);
const prefix = "http://localhost/"

async function UpdateTask(arn, data) {
    return await fetch(prefix + 'task/' + arn, {
        method: "POST",
        body: JSON.stringify(data),
    })
        .then((res) => res.json())
        .catch((e) => {
            error: e
        });
}

function Button(props) {
    return html`<button onclick="${() => UpdateTask(props.taskArn, props.payload)}">
            ${props.text}
        </button>`;
}

function TaskCard(props) {
    return html`
        <div class='task' style='border: 1px solid #ff0000; margin: 2px; max-width: 400px; display: inline-block'>
            <div>TaskArn: ${props.hypatia.taskArn}</div>
            <div>
                Protection: ${props.hypatia.taskProtectionEnabled && 'enabled' || 'disabled'} <br/>
                Expires: ${props.hypatia.taskProtectionExpiry} <br/>
                <${Button} text="enable" payload="${{taskProtectionEnabled:true}}" taskArn="${props.hypatia.taskArn}"/>
                <${Button} text="disable" payload="${{taskProtectionEnabled:false}}" taskArn="${props.hypatia.taskArn}"/>
            </div>
            <div>
                remote health: ${props.hypatia.remoteHealth}<br/>
                <a href="">Enable</a> <a href="">Disable</a><br/>
                local health: ${props.hypatia.localHealth}<br/>
                <a href="">Enable</a> <a href="">Disable</a>
            </div>
        </div>`;
}

function InstanceCard(props) {
    if (!props.tasks) {
        return html`
            <div></div>`;
    }
    let content = props.tasks.map((x) => html`
        <${TaskCard} hypatia="${x}"/>`)
    return html`
        <div class="instance" style="border: 1px solid blue; padding: 2px;">
            Instance Id: ${props.instance} <br/>
            ${content}
        </div>
    `;
}

function ClusterCard(props) {
    let instances = props.tasks.reduce((acc, task) => {
        if (!acc[task.ec2Instance]) {
            acc[task.ec2Instance] = [];
        }
        acc[task.ec2Instance].push(task);
        return acc;
    }, {});
    return Object.keys(instances).map((i) => {
        return html`
            <${InstanceCard} instance="${i}" tasks="${instances[i]}"/>`;
    });
}

fetch(prefix + "tasks")
    .then((res) => res.json())
    .then((data) => {
        let promises = data.tasks.map((t) => {
            return fetch(prefix + "task/" + t)
                .then((res) => res.json());
        });
        return Promise.all(promises);
    })
    .then((output) =>
        render(html`
            <${ClusterCard} tasks="${output}"/>`, document.body));