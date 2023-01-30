const fs = require("fs");
const path = require("path");
const tester = require("circom").tester;
const assert = require('assert');
const circomlib = require("circomlib");
const smt = require("circomlib").smt;

describe("merkletreetree circom-proof-verifier", function () {
    this.timeout(0);
    const nLevels = 4;

    let circuit;
    let circuitPath = path.join(__dirname, "circuits", "mt-proof-verifier-main.test.circom");
    before( async() => {
        const circuitCode = `
            include "smt-proof-verifier_test.circom";
            component main = SMTVerifierTest(4);
        `;
        fs.writeFileSync(circuitPath, circuitCode, "utf8");

        circuit = await tester(circuitPath, {reduceConstraints:false});
        await circuit.loadConstraints();
        console.log("Constraints: " + circuit.constraints.length + "\n");
    });

    after( async() => {
        fs.unlinkSync(circuitPath);
    });

    let inputsVerifier, inputsVerifierNonExistence;

    before("generate smt-verifier js inputs", async () => {
        let tree = await smt.newMemEmptyTrie();
        await tree.insert(1, 11);
        await tree.insert(2, 22);
        await tree.insert(3, 33);
        await tree.insert(4, 44);
        const res = await tree.find(2);
        assert(res.found);
        let root = tree.root;

        let siblings = res.siblings;
        while (siblings.length < nLevels) {
            siblings.push("0");
        };

        inputsVerifier = {
            "fnc": 0,
            "key": 2,
            "value": 22,
            "siblings": siblings,
            "root": root,
        };

        const res2 = await tree.find(5);
        assert(!res2.found);
        let siblings2 = res2.siblings;
        while (siblings2.length < nLevels) {
            siblings2.push("0");
        };
        inputsVerifierNonExistence = {
            "fnc": 1,
            "oldKey": 1,
            "oldValue": 11,
            "key": 5,
            "value": 11,
            "siblings": siblings2,
            "root": root,
        };
    });

    it("Test smt-verifier proof of existence go inputs", async () => {
        // fromGo is a json CircomVerifierProof generated from Go code using
        // https://github.com/vocdoni/arbo
        let rawdata = fs.readFileSync('go-data-generator/go-smt-verifier-inputs.json');
        let fromGo = JSON.parse(rawdata);
        inputsVerifier=fromGo;
        // console.log("smtverifier js inputs:\n", inputsVerifier);

        const witness = await circuit.calculateWitness(inputsVerifier);
        await circuit.checkConstraints(witness);
    });
    it("Test smt-verifier proof of non-existence go inputs", async () => {
        // fromGo is a json CircomVerifierProof generated from Go code using
        // https://github.com/vocdoni/arbo
        let rawdata = fs.readFileSync('go-data-generator/go-smt-verifier-non-existence-inputs.json');
        let fromGo = JSON.parse(rawdata);
        inputsVerifierNonExistence=fromGo;
        // console.log("smtverifier js inputs:\n", inputsVerifierNonExistence);

        const witness = await circuit.calculateWitness(inputsVerifierNonExistence);
        await circuit.checkConstraints(witness);
    });
});
