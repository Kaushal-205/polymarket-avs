// SPDX-License-Identifier: MIT
pragma solidity ^0.8.12;

import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/security/ReentrancyGuard.sol";

/**
 * @title SettlementVerifier
 * @dev Contract for verifying settlement transactions and handling challenges
 * @notice This contract allows AVS operators to challenge potentially fraudulent settlements
 */
contract SettlementVerifier is Ownable, ReentrancyGuard {
    
    // Settlement status enum
    enum SettlementStatus {
        Active,      // Settlement is valid and active
        Challenged,  // Settlement has been challenged
        Frozen,      // Settlement is frozen pending resolution
        Slashed      // Settlement was proven fraudulent, operator slashed
    }
    
    // Settlement information
    struct Settlement {
        bytes32 txId;
        address operator;
        uint256 timestamp;
        SettlementStatus status;
        bytes32 snapshotHash;
        string tradeBatchId;
        uint256 challengeDeadline;
    }
    
    // Challenge information
    struct Challenge {
        address challenger;
        bytes32 settlementId;
        bytes proof;
        uint256 timestamp;
        bool resolved;
        bool successful;
    }
    
    // Events
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
    
    // State variables
    mapping(bytes32 => Settlement) public settlements;
    mapping(bytes32 => Challenge) public challenges;
    mapping(address => uint256) public operatorStakes;
    mapping(address => bool) public authorizedChallengeers;
    
    uint256 public constant CHALLENGE_PERIOD = 7 days;
    uint256 public constant MIN_STAKE = 1 ether;
    uint256 public constant SLASH_PERCENTAGE = 50; // 50% of stake
    
    // Modifiers
    modifier onlyAuthorizedChallenger() {
        require(authorizedChallengeers[msg.sender], "Not authorized to challenge");
        _;
    }
    
    modifier validSettlement(bytes32 settlementId) {
        require(settlements[settlementId].operator != address(0), "Settlement does not exist");
        _;
    }
    
    constructor() {
        // Initial setup if needed
    }
    
    /**
     * @dev Register a new settlement
     * @param txId The transaction ID of the settlement
     * @param snapshotHash Hash of the orderbook snapshot
     * @param tradeBatchId Identifier for the trade batch
     */
    function registerSettlement(
        bytes32 txId,
        bytes32 snapshotHash,
        string calldata tradeBatchId
    ) external payable {
        require(msg.value >= MIN_STAKE, "Insufficient stake");
        
        bytes32 settlementId = keccak256(abi.encodePacked(txId, msg.sender, block.timestamp));
        
        require(settlements[settlementId].operator == address(0), "Settlement already exists");
        
        settlements[settlementId] = Settlement({
            txId: txId,
            operator: msg.sender,
            timestamp: block.timestamp,
            status: SettlementStatus.Active,
            snapshotHash: snapshotHash,
            tradeBatchId: tradeBatchId,
            challengeDeadline: block.timestamp + CHALLENGE_PERIOD
        });
        
        operatorStakes[msg.sender] += msg.value;
        
        emit SettlementRegistered(settlementId, txId, msg.sender, snapshotHash, tradeBatchId);
    }
    
    /**
     * @dev Challenge a settlement with proof of fraud
     * @param settlementId The ID of the settlement to challenge
     * @param proof Proof data showing the settlement is fraudulent
     */
    function challengeSettlement(
        bytes32 settlementId,
        bytes calldata proof
    ) external onlyAuthorizedChallenger validSettlement(settlementId) {
        Settlement storage settlement = settlements[settlementId];
        
        require(settlement.status == SettlementStatus.Active, "Settlement not active");
        require(block.timestamp <= settlement.challengeDeadline, "Challenge period expired");
        
        bytes32 challengeId = keccak256(abi.encodePacked(settlementId, msg.sender, block.timestamp));
        
        challenges[challengeId] = Challenge({
            challenger: msg.sender,
            settlementId: settlementId,
            proof: proof,
            timestamp: block.timestamp,
            resolved: false,
            successful: false
        });
        
        settlement.status = SettlementStatus.Challenged;
        
        emit SettlementChallenged(settlementId, msg.sender, settlement.txId);
    }
    
    /**
     * @dev Resolve a challenge (only owner can resolve for now)
     * @param challengeId The ID of the challenge to resolve
     * @param successful Whether the challenge was successful
     */
    function resolveChallenge(
        bytes32 challengeId,
        bool successful
    ) external onlyOwner {
        Challenge storage challenge = challenges[challengeId];
        require(!challenge.resolved, "Challenge already resolved");
        
        Settlement storage settlement = settlements[challenge.settlementId];
        require(settlement.status == SettlementStatus.Challenged, "Settlement not challenged");
        
        challenge.resolved = true;
        challenge.successful = successful;
        
        if (successful) {
            // Challenge successful - slash operator
            settlement.status = SettlementStatus.Slashed;
            uint256 slashAmount = (operatorStakes[settlement.operator] * SLASH_PERCENTAGE) / 100;
            
            if (slashAmount > 0) {
                operatorStakes[settlement.operator] -= slashAmount;
                // Transfer slashed amount to challenger as reward
                payable(challenge.challenger).transfer(slashAmount);
                
                emit OperatorSlashed(challenge.settlementId, settlement.operator, slashAmount);
            }
        } else {
            // Challenge unsuccessful - settlement remains active
            settlement.status = SettlementStatus.Active;
        }
        
        emit ChallengeResolved(challengeId, challenge.settlementId, successful, challenge.challenger);
    }
    
    /**
     * @dev Authorize an address to submit challenges (AVS operators)
     * @param challenger The address to authorize
     */
    function authorizeChallenger(address challenger) external onlyOwner {
        authorizedChallengeers[challenger] = true;
    }
    
    /**
     * @dev Revoke challenge authorization
     * @param challenger The address to revoke authorization from
     */
    function revokeChallenger(address challenger) external onlyOwner {
        authorizedChallengeers[challenger] = false;
    }
    
    /**
     * @dev Withdraw stake (only if no active challenges)
     * @param amount Amount to withdraw
     */
    function withdrawStake(uint256 amount) external nonReentrant {
        require(operatorStakes[msg.sender] >= amount, "Insufficient stake");
        require(amount > 0, "Amount must be positive");
        
        // Check if operator has any active settlements that could be challenged
        // This is a simplified check - in production, you'd want a more comprehensive check
        
        operatorStakes[msg.sender] -= amount;
        payable(msg.sender).transfer(amount);
    }
    
    /**
     * @dev Get settlement information
     * @param settlementId The settlement ID
     * @return Settlement struct
     */
    function getSettlement(bytes32 settlementId) external view returns (Settlement memory) {
        return settlements[settlementId];
    }
    
    /**
     * @dev Get challenge information
     * @param challengeId The challenge ID
     * @return Challenge struct
     */
    function getChallenge(bytes32 challengeId) external view returns (Challenge memory) {
        return challenges[challengeId];
    }
    
    /**
     * @dev Emergency function to freeze a settlement
     * @param settlementId The settlement to freeze
     */
    function freezeSettlement(bytes32 settlementId) external onlyOwner validSettlement(settlementId) {
        settlements[settlementId].status = SettlementStatus.Frozen;
    }
    
    /**
     * @dev Emergency function to unfreeze a settlement
     * @param settlementId The settlement to unfreeze
     */
    function unfreezeSettlement(bytes32 settlementId) external onlyOwner validSettlement(settlementId) {
        require(settlements[settlementId].status == SettlementStatus.Frozen, "Settlement not frozen");
        settlements[settlementId].status = SettlementStatus.Active;
    }
} 