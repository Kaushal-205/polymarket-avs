// SPDX-License-Identifier: MIT
pragma solidity ^0.8.12;

import "forge-std/Test.sol";
import "../src/l1-contracts/SettlementVerifier.sol";

contract SettlementVerifierTest is Test {
    SettlementVerifier public verifier;
    
    address public owner;
    address public operator;
    address public challenger;
    address public user;
    
    bytes32 public constant TEST_TX_ID = keccak256("test-tx-id");
    bytes32 public constant TEST_SNAPSHOT_HASH = keccak256("test-snapshot-hash");
    string public constant TEST_TRADE_BATCH_ID = "test-batch-001";
    
    event SettlementRegistered(
        bytes32 indexed settlementId,
        bytes32 indexed txId,
        address indexed operator,
        bytes32 snapshotHash,
        string tradeBatchId
    );
    
    event SettlementChallenged(
        bytes32 indexed settlementId,
        address indexed challenger,
        bytes32 indexed txId
    );
    
    event ChallengeResolved(
        bytes32 indexed challengeId,
        bytes32 indexed settlementId,
        bool successful,
        address indexed challenger
    );
    
    event OperatorSlashed(
        bytes32 indexed settlementId,
        address indexed operator,
        uint256 slashAmount
    );
    
    function setUp() public {
        owner = address(this);
        operator = vm.addr(1);
        challenger = vm.addr(2);
        user = vm.addr(3);
        
        verifier = new SettlementVerifier();
        
        // Authorize challenger
        verifier.authorizeChallenger(challenger);
        
        // Give test accounts some ETH
        vm.deal(operator, 10 ether);
        vm.deal(challenger, 10 ether);
        vm.deal(user, 10 ether);
    }
    
    function testRegisterSettlement() public {
        vm.startPrank(operator);
        
        bytes32 expectedSettlementId = keccak256(abi.encodePacked(TEST_TX_ID, operator, block.timestamp));
        
        vm.expectEmit(true, true, true, true);
        emit SettlementRegistered(expectedSettlementId, TEST_TX_ID, operator, TEST_SNAPSHOT_HASH, TEST_TRADE_BATCH_ID);
        
        verifier.registerSettlement{value: 2 ether}(TEST_TX_ID, TEST_SNAPSHOT_HASH, TEST_TRADE_BATCH_ID);
        
        SettlementVerifier.Settlement memory settlement = verifier.getSettlement(expectedSettlementId);
        
        assertEq(settlement.txId, TEST_TX_ID);
        assertEq(settlement.operator, operator);
        assertEq(settlement.snapshotHash, TEST_SNAPSHOT_HASH);
        assertEq(settlement.tradeBatchId, TEST_TRADE_BATCH_ID);
        assertEq(uint256(settlement.status), uint256(SettlementVerifier.SettlementStatus.Active));
        assertEq(settlement.challengeDeadline, block.timestamp + 7 days);
        
        assertEq(verifier.operatorStakes(operator), 2 ether);
        
        vm.stopPrank();
    }
    
    function testRegisterSettlementFailsWithInsufficientStake() public {
        vm.startPrank(operator);
        
        vm.expectRevert("Insufficient stake");
        verifier.registerSettlement{value: 0.5 ether}(TEST_TX_ID, TEST_SNAPSHOT_HASH, TEST_TRADE_BATCH_ID);
        
        vm.stopPrank();
    }
    
    function testChallengeSettlement() public {
        // First register a settlement
        vm.startPrank(operator);
        verifier.registerSettlement{value: 2 ether}(TEST_TX_ID, TEST_SNAPSHOT_HASH, TEST_TRADE_BATCH_ID);
        vm.stopPrank();
        
        bytes32 settlementId = keccak256(abi.encodePacked(TEST_TX_ID, operator, block.timestamp));
        
        // Challenge the settlement
        vm.startPrank(challenger);
        bytes memory proof = abi.encode("fraud-proof-data");
        
        vm.expectEmit(true, true, true, true);
        emit SettlementChallenged(settlementId, challenger, TEST_TX_ID);
        
        verifier.challengeSettlement(settlementId, proof);
        
        SettlementVerifier.Settlement memory settlement = verifier.getSettlement(settlementId);
        assertEq(uint256(settlement.status), uint256(SettlementVerifier.SettlementStatus.Challenged));
        
        vm.stopPrank();
    }
    
    function testChallengeSettlementFailsForUnauthorizedChallenger() public {
        // First register a settlement
        vm.startPrank(operator);
        verifier.registerSettlement{value: 2 ether}(TEST_TX_ID, TEST_SNAPSHOT_HASH, TEST_TRADE_BATCH_ID);
        vm.stopPrank();
        
        bytes32 settlementId = keccak256(abi.encodePacked(TEST_TX_ID, operator, block.timestamp));
        
        // Try to challenge with unauthorized user
        vm.startPrank(user);
        bytes memory proof = abi.encode("fraud-proof-data");
        
        vm.expectRevert("Not authorized to challenge");
        verifier.challengeSettlement(settlementId, proof);
        
        vm.stopPrank();
    }
    
    function testResolveSuccessfulChallenge() public {
        // Register settlement
        vm.startPrank(operator);
        verifier.registerSettlement{value: 2 ether}(TEST_TX_ID, TEST_SNAPSHOT_HASH, TEST_TRADE_BATCH_ID);
        vm.stopPrank();
        
        bytes32 settlementId = keccak256(abi.encodePacked(TEST_TX_ID, operator, block.timestamp));
        
        // Challenge settlement
        vm.startPrank(challenger);
        bytes memory proof = abi.encode("fraud-proof-data");
        verifier.challengeSettlement(settlementId, proof);
        vm.stopPrank();
        
        bytes32 challengeId = keccak256(abi.encodePacked(settlementId, challenger, block.timestamp));
        
        uint256 initialChallengerBalance = challenger.balance;
        uint256 initialOperatorStake = verifier.operatorStakes(operator);
        
        // Resolve challenge as successful
        vm.expectEmit(true, true, true, true);
        emit ChallengeResolved(challengeId, settlementId, true, challenger);
        
        vm.expectEmit(true, true, false, true);
        emit OperatorSlashed(settlementId, operator, 1 ether); // 50% of 2 ether
        
        verifier.resolveChallenge(challengeId, true);
        
        // Check settlement status
        SettlementVerifier.Settlement memory settlement = verifier.getSettlement(settlementId);
        assertEq(uint256(settlement.status), uint256(SettlementVerifier.SettlementStatus.Slashed));
        
        // Check challenge resolved
        SettlementVerifier.Challenge memory challenge = verifier.getChallenge(challengeId);
        assertTrue(challenge.resolved);
        assertTrue(challenge.successful);
        
        // Check operator stake reduced
        assertEq(verifier.operatorStakes(operator), initialOperatorStake - 1 ether);
        
        // Check challenger received reward
        assertEq(challenger.balance, initialChallengerBalance + 1 ether);
    }
    
    function testResolveUnsuccessfulChallenge() public {
        // Register settlement
        vm.startPrank(operator);
        verifier.registerSettlement{value: 2 ether}(TEST_TX_ID, TEST_SNAPSHOT_HASH, TEST_TRADE_BATCH_ID);
        vm.stopPrank();
        
        bytes32 settlementId = keccak256(abi.encodePacked(TEST_TX_ID, operator, block.timestamp));
        
        // Challenge settlement
        vm.startPrank(challenger);
        bytes memory proof = abi.encode("fraud-proof-data");
        verifier.challengeSettlement(settlementId, proof);
        vm.stopPrank();
        
        bytes32 challengeId = keccak256(abi.encodePacked(settlementId, challenger, block.timestamp));
        
        uint256 initialOperatorStake = verifier.operatorStakes(operator);
        
        // Resolve challenge as unsuccessful
        vm.expectEmit(true, true, true, true);
        emit ChallengeResolved(challengeId, settlementId, false, challenger);
        
        verifier.resolveChallenge(challengeId, false);
        
        // Check settlement status back to active
        SettlementVerifier.Settlement memory settlement = verifier.getSettlement(settlementId);
        assertEq(uint256(settlement.status), uint256(SettlementVerifier.SettlementStatus.Active));
        
        // Check challenge resolved
        SettlementVerifier.Challenge memory challenge = verifier.getChallenge(challengeId);
        assertTrue(challenge.resolved);
        assertFalse(challenge.successful);
        
        // Check operator stake unchanged
        assertEq(verifier.operatorStakes(operator), initialOperatorStake);
    }
    
    function testChallengeAfterDeadlineExpired() public {
        // Register settlement
        vm.startPrank(operator);
        verifier.registerSettlement{value: 2 ether}(TEST_TX_ID, TEST_SNAPSHOT_HASH, TEST_TRADE_BATCH_ID);
        vm.stopPrank();
        
        bytes32 settlementId = keccak256(abi.encodePacked(TEST_TX_ID, operator, block.timestamp));
        
        // Fast forward past challenge deadline
        vm.warp(block.timestamp + 8 days);
        
        // Try to challenge after deadline
        vm.startPrank(challenger);
        bytes memory proof = abi.encode("fraud-proof-data");
        
        vm.expectRevert("Challenge period expired");
        verifier.challengeSettlement(settlementId, proof);
        
        vm.stopPrank();
    }
    
    function testWithdrawStake() public {
        // Register settlement and add stake
        vm.startPrank(operator);
        verifier.registerSettlement{value: 2 ether}(TEST_TX_ID, TEST_SNAPSHOT_HASH, TEST_TRADE_BATCH_ID);
        
        uint256 initialBalance = operator.balance;
        
        // Withdraw 1 ether
        verifier.withdrawStake(1 ether);
        
        assertEq(verifier.operatorStakes(operator), 1 ether);
        assertEq(operator.balance, initialBalance + 1 ether);
        
        vm.stopPrank();
    }
    
    function testWithdrawStakeFailsWithInsufficientStake() public {
        vm.startPrank(operator);
        
        vm.expectRevert("Insufficient stake");
        verifier.withdrawStake(1 ether);
        
        vm.stopPrank();
    }
    
    function testAuthorizeAndRevokeChallenger() public {
        address newChallenger = vm.addr(4);
        
        // Initially not authorized
        assertFalse(verifier.authorizedChallengeers(newChallenger));
        
        // Authorize
        verifier.authorizeChallenger(newChallenger);
        assertTrue(verifier.authorizedChallengeers(newChallenger));
        
        // Revoke
        verifier.revokeChallenger(newChallenger);
        assertFalse(verifier.authorizedChallengeers(newChallenger));
    }
    
    function testFreezeAndUnfreezeSettlement() public {
        // Register settlement
        vm.startPrank(operator);
        verifier.registerSettlement{value: 2 ether}(TEST_TX_ID, TEST_SNAPSHOT_HASH, TEST_TRADE_BATCH_ID);
        vm.stopPrank();
        
        bytes32 settlementId = keccak256(abi.encodePacked(TEST_TX_ID, operator, block.timestamp));
        
        // Freeze settlement
        verifier.freezeSettlement(settlementId);
        
        SettlementVerifier.Settlement memory settlement = verifier.getSettlement(settlementId);
        assertEq(uint256(settlement.status), uint256(SettlementVerifier.SettlementStatus.Frozen));
        
        // Unfreeze settlement
        verifier.unfreezeSettlement(settlementId);
        
        settlement = verifier.getSettlement(settlementId);
        assertEq(uint256(settlement.status), uint256(SettlementVerifier.SettlementStatus.Active));
    }
} 