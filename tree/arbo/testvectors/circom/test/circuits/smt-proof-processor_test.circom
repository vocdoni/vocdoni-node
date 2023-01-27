include "../../node_modules/circomlib/circuits/comparators.circom";
include "../../node_modules/circomlib/circuits/poseidon.circom";
include "../../node_modules/circomlib/circuits/smt/smtprocessor.circom";

template SMTProcessorTest(nLevels) {
	signal input newKey;
	signal input newValue;
	signal private input oldKey;
	signal private input oldValue;
	signal private input isOld0;
	signal private input siblings[nLevels];
	signal input oldRoot;
	signal input newRoot;

	component smtProcessor = SMTProcessor(nLevels);
	smtProcessor.oldRoot <== oldRoot;
	smtProcessor.newRoot <== newRoot;
	for (var i=0; i<nLevels; i++) {
		smtProcessor.siblings[i] <== siblings[i];
	}
	smtProcessor.oldKey <== oldKey;
	smtProcessor.oldValue <== oldValue;
	smtProcessor.isOld0 <== isOld0;
	smtProcessor.newKey <== newKey;
	smtProcessor.newValue <== newValue;
	smtProcessor.fnc[0] <== 1;
	smtProcessor.fnc[1] <== 0;
}
